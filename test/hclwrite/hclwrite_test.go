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
	"github.com/zclconf/go-cty-debug/ctydebug"
	"github.com/zclconf/go-cty/cty"
)

func TestHCLWrite(t *testing.T) {
	type testcase struct {
		name string
		hcl  fmt.Stringer
		want string
	}

	block := func(name string, builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock(name, builders...)
	}
	hcl := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildHCL(builders...)
	}
	labels := hclwrite.Labels
	expr := hclwrite.Expression
	attr := func(name, expr string) hclwrite.BlockBuilder {
		return hclwrite.AttributeValue(t, name, expr)
	}
	str := hclwrite.String
	number := hclwrite.NumberInt
	boolean := hclwrite.Boolean

	tcases := []testcase{
		{
			name: "empty block",
			hcl:  block("test"),
			want: `
			  test {
			  }
			`,
		},
		{
			name: "block with multiple attributes",
			hcl: block("test",
				str("str", "test"),
				number("num", 666),
				boolean("bool", true),
				expr("expr_a", "local.name"),
				expr("expr_b", "local.name"),
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
			hcl: block("test",
				attr("team", `{ members = ["aaa"] }`),
				attr("nesting", `{ first = { second = { "hi": 666 } } }`),
				attr("list", `[1, 2, 3]`),
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
			hcl: block("test",
				labels("label"),
				str("str", "labeltest"),
			),
			want: `
			  test "label" {
			    str    = "labeltest"
			  }
			`,
		},
		{
			name: "empty block with one label",
			hcl: block("test",
				labels("label"),
			),
			want: `
			  test "label" {
			  }
			`,
		},
		{
			name: "block multiple labels",
			hcl: block("test",
				labels("label", "label2"),
				str("str", "labelstest"),
			),
			want: `
			  test "label" "label2" {
			    str    = "labelstest"
			  }
			`,
		},
		{
			name: "block nesting",
			hcl: block("test",
				str("str", "level1"),
				block("nested",
					str("str", "level2"),
					block("yet_more_nesting",
						str("str", "level3"),
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
			hcl: block("test",
				labels("label"),
				str("str", "level1"),
				block("nested",
					labels("label1", "label2"),
					str("str", "level2"),
					block("yet_more_nesting",
						labels("label1", "label2", "label3"),
						str("str", "level3"),
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
			hcl: hcl(
				block("b",
					labels("label1", "label2"),
					str("str", "level2"),
				),
				block("a",
					labels("label"),
					str("str", "level1"),
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
			hcl: hcl(
				boolean("rootbool", true),
				number("rootnum", 666),
				str("rootstr", "hi"),
				block("b",
					labels("label1", "label2"),
					str("str", "level2"),
				),
				block("a",
					labels("label"),
					str("str", "level1"),
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
			hcl: hcl(
				block("a",
					labels("label"),
					str("str", "level1"),
				),
				boolean("rootbool", true),
				number("rootnum", 666),
				str("rootstr", "hi"),
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
			hcl: hcl(
				block("stack",
					expr("before", `["/stack/a", "/stack/b"]`),
					expr("after", `["/stack/c"]`),
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
