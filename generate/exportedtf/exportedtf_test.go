// Copyright 2022 Mineiros GmbH
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

package exportedtf_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/generate/exportedtf"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/hclwrite"
	"github.com/mineiros-io/terramate/test/sandbox"
	"github.com/rs/zerolog"
)

func TestLoadExportedTerraform(t *testing.T) {
	type (
		hclconfig struct {
			path string
			add  fmt.Stringer
		}
		result struct {
			name string
			hcl  fmt.Stringer
		}
		testcase struct {
			name    string
			stack   string
			configs []hclconfig
			want    []result
			wantErr error
		}
	)

	hcldoc := func(blocks ...*hclwrite.Block) hclwrite.HCL {
		return hclwrite.NewHCL(blocks...)
	}
	exportAsTerraform := func(label string, builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		b := hclwrite.BuildBlock("export_as_terraform", builders...)
		b.AddLabel(label)
		return b
	}
	block := func(name string, builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock(name, builders...)
	}
	globals := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("globals", builders...)
	}
	attr := func(name, expr string) hclwrite.BlockBuilder {
		t.Helper()
		return hclwrite.AttributeValue(t, name, expr)
	}
	expr := hclwrite.Expression
	str := hclwrite.String
	number := hclwrite.NumberInt
	boolean := hclwrite.Boolean

	tcases := []testcase{
		{
			name:  "no exported terraform",
			stack: "/stack",
		},
		{
			name:  "empty export_as_terraform block generates empty code",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add:  exportAsTerraform("empty"),
				},
			},
			want: []result{
				{
					name: "empty",
					hcl:  hcldoc(),
				},
			},
		},
		{
			name:  "exported terraform on stack with single empty block",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: exportAsTerraform("emptytest",
						block("empty"),
					),
				},
			},
			want: []result{
				{
					name: "emptytest",
					hcl:  block("empty"),
				},
			},
		},
		{
			name:  "exported terraform on stack with single block",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: globals(
						str("some_string", "string"),
						number("some_number", 777),
						boolean("some_bool", true),
					),
				},
				{
					path: "/stack",
					add: exportAsTerraform("test",
						block("testblock",
							expr("bool", "global.some_bool"),
							expr("number", "global.some_number"),
							expr("string", "global.some_string"),
							expr("obj", `{
								string = global.some_string
								number = global.some_number
								bool = global.some_bool
							}`),
						),
					),
				},
			},
			want: []result{
				{
					name: "test",
					hcl: block("testblock",
						boolean("bool", true),
						number("number", 777),
						str("string", "string"),
						attr("obj", `{
							bool   = true
							number = 777
							string = "string"
						}`),
					),
				},
			},
		},
		{
			name:  "exported terraform on stack with single nested block",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: globals(
						str("some_string", "string"),
						number("some_number", 777),
						boolean("some_bool", true),
					),
				},
				{
					path: "/stack",
					add: exportAsTerraform("nesting",
						block("block1",
							expr("bool", "global.some_bool"),
							block("block2",
								expr("number", "global.some_number"),
								block("block3",
									expr("string", "global.some_string"),
									expr("obj", `{
										string = global.some_string
										number = global.some_number
										bool = global.some_bool
									}`),
								),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "nesting",
					hcl: block("block1",
						boolean("bool", true),
						block("block2",
							number("number", 777),
							block("block3",
								str("string", "string"),
								attr("obj", `{
									bool   = true
									number = 777
									string = "string"
								}`),
							),
						),
					),
				},
			},
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			s := sandbox.New(t)
			stackEntry := s.CreateStack(tcase.stack)
			stack := stackEntry.Load()

			for _, cfg := range tcase.configs {
				path := filepath.Join(s.RootDir(), cfg.path)
				test.AppendFile(t, path, config.Filename, cfg.add.String())
			}

			meta := stack.Meta()
			globals := s.LoadStackGlobals(meta)
			res, err := exportedtf.Load(s.RootDir(), meta, globals)
			assert.IsError(t, err, tcase.wantErr)

			got := res.ExportedCode()

			for _, res := range tcase.want {
				gothcl, ok := got[res.name]
				if !ok {
					t.Fatalf("want hcl code for %q, got: %v", res.name, got)
				}
				gotcode := gothcl.String()
				wantcode := res.hcl.String()

				assertHCLEquals(t, gotcode, wantcode)

				delete(got, res.name)
			}

			assert.EqualInts(t, 0, len(got), "got unexpected exported code: %v", got)
		})
	}
}

func assertHCLEquals(t *testing.T, got string, want string) {
	t.Helper()

	const trimmedChars = "\n "

	got = strings.Trim(got, trimmedChars)
	want = strings.Trim(want, trimmedChars)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Error("generated code doesn't match expectation")
		t.Errorf("want:\n%q", want)
		t.Errorf("got:\n%q", got)
		t.Fatalf("diff:\n%s", diff)
	}
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
