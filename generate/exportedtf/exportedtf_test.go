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
	terraform := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("terraform", builders...)
	}
	globals := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("globals", builders...)
	}
	attr := func(name, expr string) hclwrite.BlockBuilder {
		t.Helper()
		return hclwrite.AttributeValue(t, name, expr)
	}
	labels := func(labels ...string) hclwrite.BlockBuilder {
		return hclwrite.Labels(labels...)
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
			name:  "exported terraform on stack using try and labeled block",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: globals(
						attr("obj", `{
							field_a = "a"
							field_b = "b"
							field_c = "c"
						}`),
					),
				},
				{
					path: "/stack",
					add: exportAsTerraform("test",
						block("labeled",
							labels("label1", "label2"),
							expr("field_a", "try(global.obj.field_a, null)"),
							expr("field_b", "try(global.obj.field_b, null)"),
							expr("field_c", "try(global.obj.field_c, null)"),
							expr("field_d", "try(global.obj.field_d, null)"),
						),
					),
				},
			},
			want: []result{
				{
					name: "test",
					hcl: block("labeled",
						labels("label1", "label2"),
						str("field_a", "a"),
						str("field_b", "b"),
						str("field_c", "c"),
						attr("field_d", "null"),
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
		{
			name:  "multiple exported terraform blocks on stack",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: globals(
						str("some_string", "string"),
						number("some_number", 666),
						boolean("some_bool", true),
					),
				},
				{
					path: "/stack",
					add: hcldoc(
						exportAsTerraform("exported_one",
							block("block1",
								expr("bool", "global.some_bool"),
								block("block2",
									expr("number", "global.some_number"),
								),
							),
						),
						exportAsTerraform("exported_two",
							block("yay",
								expr("data", "global.some_string"),
							),
						),
						exportAsTerraform("exported_three",
							block("something",
								expr("number", "global.some_number"),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "exported_one",
					hcl: block("block1",
						boolean("bool", true),
						block("block2",
							number("number", 666),
						),
					),
				},
				{
					name: "exported_two",
					hcl: block("yay",
						str("data", "string"),
					),
				},
				{
					name: "exported_three",
					hcl: block("something",
						number("number", 666),
					),
				},
			},
		},
		{
			name:  "exported terraform on stack parent dir",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/",
					add: globals(
						str("some_string", "string"),
						number("some_number", 777),
						boolean("some_bool", true),
					),
				},
				{
					path: "/stacks",
					add: exportAsTerraform("on_parent",
						block("on_parent_block",
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
					name: "on_parent",
					hcl: block("on_parent_block",
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
			name:  "exported terraform project root dir",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/",
					add: exportAsTerraform("root",
						block("root",
							expr("test", "terramate.path"),
						),
					),
				},
			},
			want: []result{
				{
					name: "root",
					hcl: block("root",
						str("test", "/stacks/stack"),
					),
				},
			},
		},
		{
			name:  "exporting on all dirs of the project with different names get merged",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/",
					add: globals(
						str("some_string", "string"),
						number("some_number", 777),
						boolean("some_bool", true),
					),
				},
				{
					path: "/",
					add: exportAsTerraform("on_root",
						block("on_root_block",
							expr("obj", `{
								string = global.some_string
							}`),
						),
					),
				},
				{
					path: "/stacks",
					add: exportAsTerraform("on_parent",
						block("on_parent_block",
							expr("obj", `{
								number = global.some_number
							}`),
						),
					),
				},
				{
					path: "/stacks/stack",
					add: exportAsTerraform("on_stack",
						block("on_stack_block",
							expr("obj", `{
								bool = global.some_bool
							}`),
						),
					),
				},
			},
			want: []result{
				{
					name: "on_root",
					hcl: block("on_root_block",
						attr("obj", `{
							string = "string"
						}`),
					),
				},
				{
					name: "on_parent",
					hcl: block("on_parent_block",
						attr("obj", `{
							number = 777
						}`),
					),
				},
				{
					name: "on_stack",
					hcl: block("on_stack_block",
						attr("obj", `{
							bool   = true
						}`),
					),
				},
			},
		},
		{
			name:  "specific config overrides general config",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/",
					add: hcldoc(
						exportAsTerraform("root",
							block("block",
								expr("root_stackpath", "terramate.path"),
							),
						),
						exportAsTerraform("parent",
							block("block",
								expr("root_stackpath", "terramate.path"),
							),
						),
						exportAsTerraform("stack",
							block("block",
								expr("root_stackpath", "terramate.path"),
							),
						),
					),
				},
				{
					path: "/stacks",
					add: exportAsTerraform("parent",
						block("block",
							expr("parent_stackpath", "terramate.path"),
							expr("parent_stackname", "terramate.name"),
						),
					),
				},
				{
					path: "/stacks/stack",
					add: exportAsTerraform("stack",
						block("block",
							str("overridden", "some literal data"),
						),
					),
				},
			},
			want: []result{
				{
					name: "root",
					hcl: block("block",
						str("root_stackpath", "/stacks/stack"),
					),
				},
				{
					name: "parent",
					hcl: block("block",
						str("parent_stackpath", "/stacks/stack"),
						str("parent_stackname", "stack"),
					),
				},
				{
					name: "stack",
					hcl: block("block",
						str("overridden", "some literal data"),
					),
				},
			},
		},
		{
			name:  "export block with no label on stack gives err",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks/stack",
					add: block("export_as_terraform",
						block("block",
							str("data", "some literal data"),
						),
					),
				},
			},
			wantErr: exportedtf.ErrInvalidBlock,
		},
		{
			name:  "export block with two labels on stack gives err",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks/stack",
					add: block("export_as_terraform",
						labels("one", "two"),
						block("block",
							str("data", "some literal data"),
						),
					),
				},
			},
			wantErr: exportedtf.ErrInvalidBlock,
		},
		{
			name:  "export block with empty label on stack gives err",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks/stack",
					add: block("export_as_terraform",
						labels(""),
						block("block",
							str("data", "some literal data"),
						),
					),
				},
			},
			wantErr: exportedtf.ErrInvalidBlock,
		},
		{
			name:  "export blocks with same label on same config gives err",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks/stack",
					add: hcldoc(
						exportAsTerraform("duplicated",
							str("data", "some literal data"),
						),
						exportAsTerraform("duplicated",
							str("data2", "some literal data2"),
						),
					),
				},
			},
			wantErr: exportedtf.ErrInvalidBlock,
		},
		{
			name:  "evaluation failure on stack config fails",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks/stack",
					add: hcldoc(
						exportAsTerraform("test",
							terraform(
								expr("required_version", "global.undefined"),
							),
						),
					),
				},
			},
			wantErr: exportedtf.ErrEval,
		},
		{
			name:  "valid config on stack but invalid on parent fails",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks",
					add: block("export_as_terraform",
						block("block",
							str("data", "some literal data"),
						),
					),
				},
				{
					path: "/stacks/stack",
					add: hcldoc(
						exportAsTerraform("valid",
							str("data", "some literal data"),
						),
					),
				},
			},
			wantErr: exportedtf.ErrInvalidBlock,
		},
		{
			name:  "attributes on export_as_terraform block fails",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks/stack",
					add: hcldoc(
						exportAsTerraform("test",
							str("some_attribute", "whatever"),
							terraform(
								str("required_version", "1.11"),
							),
						),
					),
				},
			},
			wantErr: exportedtf.ErrInvalidBlock,
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
