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

package genfile_test

import (
	"fmt"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/generate/genfile"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/stack"
	"github.com/mineiros-io/terramate/test"
	errtest "github.com/mineiros-io/terramate/test/errors"
	"github.com/mineiros-io/terramate/test/hclwrite"
	"github.com/mineiros-io/terramate/test/sandbox"
	"github.com/rs/zerolog"
)

func TestLoadGenerateFiles(t *testing.T) {
	type (
		hclconfig struct {
			path string
			add  fmt.Stringer
		}
		genFile struct {
			body   string
			origin string
		}
		result struct {
			name      string
			condition bool
			file      genFile
		}
		testcase struct {
			name    string
			stack   string
			configs []hclconfig
			want    []result
			wantErr error
		}
	)

	generateFile := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("generate_file", builders...)
	}
	hcldoc := hclwrite.BuildHCL
	labels := hclwrite.Labels
	expr := hclwrite.Expression
	str := hclwrite.String
	boolean := hclwrite.Boolean
	attr := func(name string, expr string) hclwrite.BlockBuilder {
		return hclwrite.AttributeValue(t, name, expr)
	}
	globals := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("globals", builders...)
	}

	tcases := []testcase{
		{
			name:  "no generation",
			stack: "/stack",
		},
		{
			name:  "dotfile is ignored",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/.test.tm",
					add: generateFile(
						labels("test"),
						str("content", "test"),
					),
				},
			},
		},
		{
			name:  "empty content attribute generates empty body",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/empty.tm",
					add: generateFile(
						labels("empty"),
						str("content", ""),
					),
				},
			},
			want: []result{
				{
					name:      "empty",
					condition: true,
					file: genFile{
						origin: "/stack/empty.tm",
						body:   "",
					},
				},
			},
		},
		{
			name:  "simple plain string",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/test.tm",
					add: generateFile(
						labels("test"),
						str("content", "test"),
					),
				},
			},
			want: []result{
				{
					name:      "test",
					condition: true,
					file: genFile{
						origin: "/stack/test.tm",
						body:   "test",
					},
				},
			},
		},
		{
			name:  "all metadata available by default",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/test.tm",
					add: generateFile(
						labels("test"),
						expr("content", `<<EOT

stacks_list=${tm_jsonencode(terramate.stacks.list)}
stack_path_abs=${terramate.stack.path.absolute}
stack_path_rel=${terramate.stack.path.relative}
stack_path_to_root=${terramate.stack.path.to_root}
stack_path_basename=${terramate.stack.path.basename}
stack_id=${tm_try(terramate.stack.id, "no-id")}
stack_name=${terramate.stack.name}
stack_description=${terramate.stack.description}
EOT`,
						)),
				},
			},
			want: []result{
				{
					name:      "test",
					condition: true,
					file: genFile{
						origin: "/stack/test.tm",
						body: `
stacks_list=["/stack"]
stack_path_abs=/stack
stack_path_rel=stack
stack_path_to_root=..
stack_path_basename=stack
stack_id=no-id
stack_name=stack
stack_description=
`,
					},
				},
			},
		},
		{
			name:  "stack.id metadata available",
			stack: "/stack:id=stack-id",
			configs: []hclconfig{
				{
					path: "/stack/test.tm",
					add: generateFile(
						labels("test"),
						expr("content", `<<EOT

stack_id=${terramate.stack.id}
EOT`,
						)),
				},
			},
			want: []result{
				{
					name:      "test",
					condition: true,
					file: genFile{
						origin: "/stack/test.tm",
						body: `
stack_id=stack-id
`,
					},
				},
			},
		},
		{
			name:  "using globals and metadata with interpolation",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/globals.tm",
					add: globals(
						str("data", "global-data"),
					),
				},
				{
					path: "/stack/test.tm",
					add: generateFile(
						labels("test"),
						str("content", "${global.data}-${terramate.stack.path.absolute}"),
					),
				},
			},
			want: []result{
				{
					name:      "test",
					condition: true,
					file: genFile{
						origin: "/stack/test.tm",
						body:   "global-data-/stack",
					},
				},
			},
		},
		{
			name:  "condition set to false",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/test.tm",
					add: generateFile(
						labels("test"),
						boolean("condition", false),
						str("content", "data"),
					),
				},
			},
			want: []result{
				{
					name:      "test",
					condition: false,
					file: genFile{
						origin: "/stack/test.tm",
						body:   "",
					},
				},
			},
		},
		{
			name:  "mixed conditions on different blocks",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/test.tm",
					add: hcldoc(
						generateFile(
							labels("test"),
							boolean("condition", false),
							str("content", "data"),
						),
						generateFile(
							labels("test2"),
							boolean("condition", true),
							str("content", "data"),
						),
					),
				},
			},
			want: []result{
				{
					name:      "test",
					condition: false,
					file: genFile{
						origin: "/stack/test.tm",
						body:   "",
					},
				},
				{
					name:      "test2",
					condition: true,
					file: genFile{
						origin: "/stack/test.tm",
						body:   "data",
					},
				},
			},
		},
		{
			name:  "condition evaluated from global",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/globals.tm",
					add: globals(
						boolean("condition", false),
					),
				},
				{
					path: "/stack/test.tm",
					add: generateFile(
						labels("test"),
						expr("condition", "global.condition"),
						str("content", "cond=${global.condition}"),
					),
				},
			},
			want: []result{
				{
					name:      "test",
					condition: false,
					file: genFile{
						origin: "/stack/test.tm",
						body:   "",
					},
				},
			},
		},
		{
			name:  "condition evaluated using try",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/test.tm",
					add: generateFile(
						labels("test"),
						expr("condition", "tm_try(global.condition, false)"),
						str("content", "whatever"),
					),
				},
			},
			want: []result{
				{
					name:      "test",
					condition: false,
					file: genFile{
						origin: "/stack/test.tm",
						body:   "",
					},
				},
			},
		},
		{
			name:  "condition evaluated using functions",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/globals.tm",
					add: globals(
						attr("list", "[1]"),
					),
				},
				{
					path: "/stack/test.tm",
					add: generateFile(
						labels("test"),
						expr("condition", "tm_length(global.list) > 0"),
						str("content", "data"),
					),
				},
			},
			want: []result{
				{
					name:      "test",
					condition: true,
					file: genFile{
						origin: "/stack/test.tm",
						body:   "data",
					},
				},
			},
		},
		{
			name:  "multiple generate_file blocks on same file",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/globals.tm",
					add: globals(
						str("data", "global-data"),
					),
				},
				{
					path: "/stack/test.tm",
					add: hcldoc(
						generateFile(
							labels("test1"),
							expr("content", "global.data"),
						),
						generateFile(
							labels("test2"),
							expr("content", "terramate.path"),
						),
						generateFile(
							labels("test3"),
							str("content", "terramate!"),
						),
					),
				},
			},
			want: []result{
				{
					name:      "test1",
					condition: true,
					file: genFile{
						origin: "/stack/test.tm",
						body:   "global-data",
					},
				},
				{
					name:      "test2",
					condition: true,
					file: genFile{
						origin: "/stack/test.tm",
						body:   "/stack",
					},
				},
				{
					name:      "test3",
					condition: true,
					file: genFile{
						origin: "/stack/test.tm",
						body:   "terramate!",
					},
				},
			},
		},
		{
			name:  "using globals and metadata with functions",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/globals.tm",
					add: globals(
						str("data", "global-data"),
					),
				},
				{
					path: "/stack/json.tm",
					add: generateFile(
						labels("test.json"),
						expr("content", "tm_jsonencode({field = global.data})"),
					),
				},
				{
					path: "/stack/yaml.tm",
					add: generateFile(
						labels("test.yml"),
						expr("content", "tm_yamlencode({field = terramate.stack.path.absolute})"),
					),
				},
			},
			want: []result{
				{
					name:      "test.json",
					condition: true,
					file: genFile{
						origin: "/stack/json.tm",
						body:   `{"field":"global-data"}`,
					},
				},
				{
					name:      "test.yml",
					condition: true,
					file: genFile{
						origin: "/stack/yaml.tm",
						body:   "\"field\": \"/stack\"\n",
					},
				},
			},
		},
		{
			name:  "hierarchical load",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/root.tm",
					add: generateFile(
						labels("root"),
						str("content", "root-${global.data}-${terramate.path}"),
					),
				},
				{
					path: "/stacks/globals.tm",
					add: globals(
						str("data", "global-data"),
					),
				},
				{
					path: "/stacks/stacks.tm",
					add: generateFile(
						labels("stacks"),
						str("content", "stacks-${global.data}-${terramate.path}"),
					),
				},
				{
					path: "/stacks/stack/stack.tm",
					add: generateFile(
						labels("stack"),
						str("content", "stack-${global.data}-${terramate.path}"),
					),
				},
			},
			want: []result{
				{
					name:      "root",
					condition: true,
					file: genFile{
						origin: "/root.tm",
						body:   "root-global-data-/stacks/stack",
					},
				},
				{
					name:      "stack",
					condition: true,
					file: genFile{
						origin: "/stacks/stack/stack.tm",
						body:   "stack-global-data-/stacks/stack",
					},
				},
				{
					name:      "stacks",
					condition: true,
					file: genFile{
						origin: "/stacks/stacks.tm",
						body:   "stacks-global-data-/stacks/stack",
					},
				},
			},
		},
		{
			name:  "content must be string",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/test.tm",
					add: generateFile(
						labels("test.yml"),
						expr("content", "5"),
					),
				},
			},
			wantErr: errors.E(genfile.ErrInvalidContentType),
		},
		{
			name:  "blocks with same label are allowed",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/test.tm",
					add: hcldoc(
						generateFile(
							labels("test.yml"),
							str("content", "test"),
						),
						generateFile(
							labels("test.yml"),
							str("content", "test2"),
						),
					),
				},
			},
			want: []result{
				{
					name:      "test.yml",
					condition: true,
					file: genFile{
						origin: "/stack/test.tm",
						body:   "test",
					},
				},
				{
					name:      "test.yml",
					condition: true,
					file: genFile{
						origin: "/stack/test.tm",
						body:   "test2",
					},
				},
			},
		},
		{
			name:  "same labels but different condition",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/test.tm",
					add: generateFile(
						labels("test"),
						str("content", "test"),
						boolean("condition", true),
					),
				},
				{
					path: "/stack/test.tm",
					add: generateFile(
						labels("test"),
						str("content", "test"),
						boolean("condition", false),
					),
				},
			},
			want: []result{
				{
					name:      "test",
					condition: false,
					file: genFile{
						origin: "/stack/test.tm",
					},
				},
				{
					name:      "test",
					condition: true,
					file: genFile{
						origin: "/stack/test.tm",
						body:   "test",
					},
				},
			},
		},
		{
			name:  "conflicting blocks on same dir",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/test.tm",
					add: hcldoc(
						generateFile(
							labels("test.yml"),
							str("content", "test"),
						),
					),
				},
				{
					path: "/stack/test2.tm",
					add: hcldoc(
						generateFile(
							labels("test.yml"),
							str("content", "test2"),
						),
					),
				},
			},
			want: []result{
				{
					name:      "test.yml",
					condition: true,
					file: genFile{
						origin: "/stack/test.tm",
						body:   "test",
					},
				},
				{
					name:      "test.yml",
					condition: true,
					file: genFile{
						origin: "/stack/test2.tm",
						body:   "test2",
					},
				},
			},
		},
		{
			name:  "conflicting blocks on different dirs",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/test.tm",
					add: hcldoc(
						generateFile(
							labels("test.yml"),
							str("content", "root"),
						),
					),
				},
				{
					path: "/stack/test.tm",
					add: hcldoc(
						generateFile(
							labels("test.yml"),
							str("content", "test"),
						),
					),
				},
			},
			want: []result{
				{
					name:      "test.yml",
					condition: true,
					file: genFile{
						origin: "/test.tm",
						body:   "root",
					},
				},
				{
					name:      "test.yml",
					condition: true,
					file: genFile{
						origin: "/stack/test.tm",
						body:   "test",
					},
				},
			},
		},
		{
			name:  "generate_file missing label",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/test.tm",
					add: hcldoc(
						generateFile(
							str("content", "root"),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name:  "generate_file with two labels",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/test.tm",
					add: hcldoc(
						generateFile(
							labels("test.yml", "test"),
							str("content", "root"),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name:  "generate_file with empty label",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/test.tm",
					add: hcldoc(
						generateFile(
							labels(""),
							str("content", "root"),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name:  "generate_file missing content attribute",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/test.tm",
					add: hcldoc(
						generateFile(
							labels("name"),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name:  "generate_file with unknown attribute",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/test.tm",
					add: hcldoc(
						generateFile(
							labels("name"),
							str("content", "data"),
							str("unknown", "data"),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name:  "generate_file fails to evaluate content",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/test.tm",
					add: hcldoc(
						generateFile(
							labels("name"),
							expr("content", "global.unknown"),
						),
					),
				},
			},
			wantErr: errors.E(genfile.ErrContentEval),
		},
		{
			name:  "generate_file fails to evaluate condition",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/test.tm",
					add: hcldoc(
						generateFile(
							labels("name"),
							expr("condition", "global.unknown"),
							str("content", "data"),
						),
					),
				},
			},
			wantErr: errors.E(genfile.ErrConditionEval),
		},
		{
			name:  "generate_file fails condition dont evaluate to boolean",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/test.tm",
					add: hcldoc(
						generateFile(
							labels("name"),
							str("condition", "not boolean"),
							str("content", "data"),
						),
					),
				},
			},
			wantErr: errors.E(genfile.ErrInvalidConditionType),
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree([]string{"s:" + tcase.stack})
			stacks := s.LoadStacks()
			projmeta := stack.NewProjectMetadata(s.RootDir(), stacks)
			stack := s.LoadStacks()[0]

			for _, cfg := range tcase.configs {
				test.AppendFile(t, s.RootDir(), cfg.path, cfg.add.String())
			}

			globals := s.LoadStackGlobals(projmeta, stack)
			got, err := genfile.Load(projmeta, stack, globals)
			errtest.Assert(t, err, tcase.wantErr)

			if len(got) != len(tcase.want) {
				for i, file := range got {
					t.Logf("got[%d] = %+v", i, file)
				}
				for i, file := range tcase.want {
					t.Logf("want[%d] = %+v", i, file)
				}
				t.Fatalf("length of got and want mismatch: got %d but want %d",
					len(got), len(tcase.want))
			}

			for i, want := range tcase.want {
				gotFile := got[i]
				gotBody := gotFile.Body()
				wantBody := want.file.body

				if gotFile.Condition() != want.condition {
					t.Fatalf("got condition %t != wanted %t", gotFile.Condition(), want.condition)
				}

				assert.EqualStrings(t,
					want.name,
					gotFile.Name(),
					"wrong name config path for generated code",
				)

				assert.EqualStrings(t,
					want.file.origin,
					gotFile.Origin(),
					"wrong origin config path for generated code",
				)

				assert.EqualStrings(t, wantBody, gotBody,
					"generated file body differs",
				)
			}
		})
	}
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
