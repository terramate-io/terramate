// Copyright 2021 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hclwrite_test

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/test/hclwrite"
	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
	"github.com/rs/zerolog"
	"github.com/zclconf/go-cty-debug/ctydebug"
	"github.com/zclconf/go-cty/cty"
)

func TestHCLWrite(t *testing.T) {
	type testcase struct {
		name string
		hcl  fmt.Stringer
		want string
	}

	tcases := []testcase{
		{
			name: "empty block",
			hcl:  Block("test"),
			want: `
			  test {
			  }
			`,
		},
		{
			name: "string contents are not quoted",
			hcl: Block("test",
				Str("str", `THIS IS ${tm_upper(global.value) + "test"} !!!`),
			),
			want: `
				test {
					str = "THIS IS ${tm_upper(global.value) + "test"} !!!"
				}
			`,
		},
		{
			name: "block with multiple attributes",
			hcl: Block("test",
				Str("str", "test"),
				Number("num", 666),
				Bool("bool", true),
				Expr("expr_a", "local.name"),
				Expr("expr_b", "local.name"),
			),
			want: `
			  test {
			    str    = "test"
			    num    = 666
			    bool   = true
			    expr_a = local.name
			    expr_b = local.name
			  }
			`,
		},
		{
			name: "block with complex attributes",
			hcl: Block("test",
				EvalExpr(t, "team", `{ members = ["aaa"] }`),
				EvalExpr(t, "nesting", `{ first = { second = { "hi": 666 } } }`),
				EvalExpr(t, "list", `[1, 2, 3]`),
			),
			want: `
			  test {
			    team    = { members = ["aaa"] }
			    nesting = { first = { second = { "hi": 666 } } }
			    list    = [1, 2, 3]
			  }
			`,
		},
		{
			name: "block with one label",
			hcl: Block("test",
				Labels("label"),
				Str("str", "labeltest"),
			),
			want: `
			  test "label" {
			    str    = "labeltest"
			  }
			`,
		},
		{
			name: "empty block with one label",
			hcl: Block("test",
				Labels("label"),
			),
			want: `
			  test "label" {
			  }
			`,
		},
		{
			name: "block multiple labels",
			hcl: Block("test",
				Labels("label", "label2"),
				Str("str", "labelstest"),
			),
			want: `
			  test "label" "label2" {
			    str    = "labelstest"
			  }
			`,
		},
		{
			name: "block nesting",
			hcl: Block("test",
				Str("str", "level1"),
				Block("nested",
					Str("str", "level2"),
					Block("yet_more_nesting",
						Str("str", "level3"),
					),
				),
			),
			want: `
			  test {
			    str = "level1"
			    nested {
			      str = "level2"
			      yet_more_nesting {
			        str = "level3"
			      }
			    }
			  }
			`,
		},
		{
			name: "block nesting with labels",
			hcl: Block("test",
				Labels("label"),
				Str("str", "level1"),
				Block("nested",
					Labels("label1", "label2"),
					Str("str", "level2"),
					Block("yet_more_nesting",
						Labels("label1", "label2", "label3"),
						Str("str", "level3"),
					),
				),
			),
			want: `
			  test "label" {
			    str = "level1"
			    nested "label1" "label2" {
			      str = "level2"
			      yet_more_nesting "label1" "label2" "label3" {
			        str = "level3"
			      }
			    }
			  }
			`,
		},
		{
			name: "multiple blocks on root doc follow order of insertion",
			hcl: Doc(
				Block("b",
					Labels("label1", "label2"),
					Str("str", "level2"),
				),
				Block("a",
					Labels("label"),
					Str("str", "level1"),
				),
			),
			want: `
			  b "label1" "label2" {
			    str = "level2"
			  }
			  a "label" {
			    str = "level1"
			  }
			`,
		},
		{
			name: "attributes on root doc with blocks",
			hcl: Doc(
				Bool("rootbool", true),
				Number("rootnum", 666),
				Str("rootstr", "hi"),
				Block("b",
					Labels("label1", "label2"),
					Str("str", "level2"),
				),
				Block("a",
					Labels("label"),
					Str("str", "level1"),
				),
			),
			want: `
			  rootbool = true
			  rootnum  = 666
			  rootstr  = "hi"
			  b "label1" "label2" {
			    str = "level2"
			  }
			  a "label" {
			    str = "level1"
			  }
			`,
		},
		{
			name: "attributes can be added after blocks",
			hcl: Doc(
				Block("a",
					Labels("label"),
					Str("str", "level1"),
				),
				Bool("rootbool", true),
				Number("rootnum", 666),
				Str("rootstr", "hi"),
			),
			want: `
			  a "label" {
			    str = "level1"
			  }
			  rootbool = true
			  rootnum  = 666
			  rootstr  = "hi"
			`,
		},
		{
			name: "terramate stack example",
			hcl: Doc(
				Block("stack",
					Expr("before", `["/stack/a", "/stack/b"]`),
					Expr("after", `["/stack/c"]`),
				),
			),
			want: `
			  stack {
			    before = ["/stack/a", "/stack/b"]
			    after  = ["/stack/c"]
			  }
			`,
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			want := hclwrite.Format(tcase.want)
			got := tcase.hcl.String()

			assertIsValidHCL(t, got)

			if diff := cmp.Diff(got, want); diff != "" {
				t.Errorf("got:\n%s", got)
				t.Errorf("want:\n%s", want)
				t.Error("diff:")
				t.Fatal(diff)
			}
		})
	}
}

func TestHCLWriteAddingAttributeValue(t *testing.T) {
	block := func(name string, builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock(name, builders...)
	}
	attr := func(name, expr string) hclwrite.BlockBuilder {
		return hclwrite.AttributeValue(t, name, expr)
	}
	const objectExpression = `{ members = ["aaa"] }`

	testblock := block("test",
		attr("team", objectExpression),
	)
	want := evaluateValExpr(t, objectExpression)
	gotAttrsValues := testblock.AttributesValues()

	assert.EqualInts(t, 1, len(gotAttrsValues))

	got := gotAttrsValues["team"]

	if diff := ctydebug.DiffValues(want, got); diff != "" {
		t.Fatal(diff)
	}
}

func assertIsValidHCL(t *testing.T, code string) {
	t.Helper()

	parser := hclparse.NewParser()
	_, diags := parser.ParseHCL([]byte(code), "")
	if diags.HasErrors() {
		t.Errorf("invalid HCL: %v", diags)
		t.Fatalf("code:\n%s", code)
	}
}

func evaluateValExpr(t *testing.T, valueExpr string) cty.Value {
	t.Helper()

	parser := hclparse.NewParser()
	res, diags := parser.ParseHCL([]byte("t = "+valueExpr), "")
	if diags.HasErrors() {
		t.Fatal(diags)
	}
	body := res.Body.(*hclsyntax.Body)

	val, diags := body.Attributes["t"].Expr.Value(nil)
	if diags.HasErrors() {
		t.Fatal(diags)
	}

	return val
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
