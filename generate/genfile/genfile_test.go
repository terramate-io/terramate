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
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/generate/genfile"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/hcl/info"
	"github.com/mineiros-io/terramate/lets"
	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/stack"
	"github.com/mineiros-io/terramate/test"
	errtest "github.com/mineiros-io/terramate/test/errors"
	infotest "github.com/mineiros-io/terramate/test/hclutils/info"
	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
	"github.com/mineiros-io/terramate/test/sandbox"
	"github.com/rs/zerolog"
)

func TestLoadGenerateFiles(t *testing.T) {
	t.Parallel()

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
					add: GenerateFile(
						Labels("test"),
						Str("content", "test"),
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
					add: GenerateFile(
						Labels("empty"),
						Str("content", ""),
					),
				},
			},
			want: []result{
				{
					name: "empty",
					file: genFile{
						body:      "",
						condition: true,
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
					add: GenerateFile(
						Labels("test"),
						Str("content", "test"),
					),
				},
			},
			want: []result{
				{
					name: "test",
					file: genFile{
						body:      "test",
						condition: true,
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
					add: GenerateFile(
						Labels("test"),
						Expr("content", `<<EOT

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
					name: "test",
					file: genFile{
						condition: true,
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
					add: GenerateFile(
						Labels("test"),
						Expr("content", `<<EOT

stack_id=${terramate.stack.id}
EOT`,
						)),
				},
			},
			want: []result{
				{
					name: "test",
					file: genFile{
						condition: true,
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
					add: Globals(
						Str("data", "global-data"),
					),
				},
				{
					path: "/stack/test.tm",
					add: GenerateFile(
						Labels("test"),
						Str("content", "${global.data}-${terramate.stack.path.absolute}"),
					),
				},
			},
			want: []result{
				{
					name: "test",
					file: genFile{
						condition: true,
						body:      "global-data-/stack",
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
					add: GenerateFile(
						Labels("test"),
						Bool("condition", false),
						Str("content", "data"),
					),
				},
			},
			want: []result{
				{
					name: "test",
					file: genFile{
						condition: false,
						body:      "",
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
					add: Doc(
						GenerateFile(
							Labels("test"),
							Bool("condition", false),
							Str("content", "data"),
						),
						GenerateFile(
							Labels("test2"),
							Bool("condition", true),
							Str("content", "data"),
						),
					),
				},
			},
			want: []result{
				{
					name: "test",
					file: genFile{
						condition: false,
						body:      "",
					},
				},
				{
					name: "test2",
					file: genFile{
						condition: true,
						body:      "data",
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
					add: Globals(
						Bool("condition", false),
					),
				},
				{
					path: "/stack/test.tm",
					add: GenerateFile(
						Labels("test"),
						Expr("condition", "global.condition"),
						Str("content", "cond=${global.condition}"),
					),
				},
			},
			want: []result{
				{
					name: "test",
					file: genFile{
						condition: false,
						body:      "",
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
					add: GenerateFile(
						Labels("test"),
						Expr("condition", "tm_try(global.condition, false)"),
						Str("content", "whatever"),
					),
				},
			},
			want: []result{
				{
					name: "test",
					file: genFile{
						condition: false,
						body:      "",
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
					add: Globals(
						EvalExpr(t, "list", "[1]"),
					),
				},
				{
					path: "/stack/test.tm",
					add: GenerateFile(
						Labels("test"),
						Expr("condition", "tm_length(global.list) > 0"),
						Str("content", "data"),
					),
				},
			},
			want: []result{
				{
					name: "test",
					file: genFile{
						condition: true,
						body:      "data",
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
					add: Globals(
						Str("data", "global-data"),
					),
				},
				{
					path: "/stack/test.tm",
					add: Doc(
						GenerateFile(
							Labels("test1"),
							Expr("content", "global.data"),
						),
						GenerateFile(
							Labels("test2"),
							Expr("content", "terramate.path"),
						),
						GenerateFile(
							Labels("test3"),
							Str("content", "terramate!"),
						),
					),
				},
			},
			want: []result{
				{
					name: "test1",
					file: genFile{
						condition: true,
						body:      "global-data",
					},
				},
				{
					name: "test2",
					file: genFile{
						condition: true,
						body:      "/stack",
					},
				},
				{
					name: "test3",
					file: genFile{
						condition: true,
						body:      "terramate!",
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
					add: Globals(
						Str("data", "global-data"),
					),
				},
				{
					path: "/stack/json.tm",
					add: GenerateFile(
						Labels("test.json"),
						Expr("content", "tm_jsonencode({field = global.data})"),
					),
				},
				{
					path: "/stack/yaml.tm",
					add: GenerateFile(
						Labels("test.yml"),
						Expr("content", "tm_yamlencode({field = terramate.stack.path.absolute})"),
					),
				},
			},
			want: []result{
				{
					name: "test.json",
					file: genFile{
						condition: true,
						body:      `{"field":"global-data"}`,
					},
				},
				{
					name: "test.yml",
					file: genFile{
						condition: true,
						body:      "\"field\": \"/stack\"\n",
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
					add: GenerateFile(
						Labels("root"),
						Str("content", "root-${global.data}-${terramate.path}"),
					),
				},
				{
					path: "/stacks/globals.tm",
					add: Globals(
						Str("data", "global-data"),
					),
				},
				{
					path: "/stacks/stacks.tm",
					add: GenerateFile(
						Labels("stacks"),
						Str("content", "stacks-${global.data}-${terramate.path}"),
					),
				},
				{
					path: "/stacks/stack/stack.tm",
					add: GenerateFile(
						Labels("stack"),
						Str("content", "stack-${global.data}-${terramate.path}"),
					),
				},
			},
			want: []result{
				{
					name: "root",
					file: genFile{
						condition: true,
						body:      "root-global-data-/stacks/stack",
					},
				},
				{
					name: "stack",
					file: genFile{
						condition: true,
						body:      "stack-global-data-/stacks/stack",
					},
				},
				{
					name: "stacks",
					file: genFile{
						condition: true,
						body:      "stacks-global-data-/stacks/stack",
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
					add: GenerateFile(
						Labels("test.yml"),
						Expr("content", "5"),
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
					add: Doc(
						GenerateFile(
							Labels("test.yml"),
							Str("content", "test"),
						),
						GenerateFile(
							Labels("test.yml"),
							Str("content", "test2"),
						),
					),
				},
			},
			want: []result{
				{
					name: "test.yml",
					file: genFile{
						condition: true,
						body:      "test",
					},
				},
				{
					name: "test.yml",
					file: genFile{
						condition: true,
						body:      "test2",
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
					add: GenerateFile(
						Labels("test"),
						Str("content", "test"),
						Bool("condition", true),
					),
				},
				{
					path: "/stack/test.tm",
					add: GenerateFile(
						Labels("test"),
						Str("content", "test"),
						Bool("condition", false),
					),
				},
			},
			want: []result{
				{
					name: "test",
					file: genFile{
						condition: false,
					},
				},
				{
					name: "test",
					file: genFile{
						condition: true,
						body:      "test",
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
					add: Doc(
						GenerateFile(
							Labels("test.yml"),
							Str("content", "test"),
						),
					),
				},
				{
					path: "/stack/test2.tm",
					add: Doc(
						GenerateFile(
							Labels("test.yml"),
							Str("content", "test2"),
						),
					),
				},
			},
			want: []result{
				{
					name: "test.yml",
					file: genFile{
						condition: true,
						body:      "test",
					},
				},
				{
					name: "test.yml",
					file: genFile{
						condition: true,
						body:      "test2",
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
					add: Doc(
						GenerateFile(
							Labels("test.yml"),
							Str("content", "root"),
						),
					),
				},
				{
					path: "/stack/test.tm",
					add: Doc(
						GenerateFile(
							Labels("test.yml"),
							Str("content", "test"),
						),
					),
				},
			},
			want: []result{
				{
					name: "test.yml",
					file: genFile{
						condition: true,
						body:      "root",
					},
				},
				{
					name: "test.yml",
					file: genFile{
						condition: true,
						body:      "test",
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
					add: Doc(
						GenerateFile(
							Str("content", "root"),
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
					add: Doc(
						GenerateFile(
							Labels("test.yml", "test"),
							Str("content", "root"),
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
					add: Doc(
						GenerateFile(
							Labels(""),
							Str("content", "root"),
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
					add: Doc(
						GenerateFile(
							Labels("name"),
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
					add: Doc(
						GenerateFile(
							Labels("name"),
							Str("content", "data"),
							Str("unknown", "data"),
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
					add: Doc(
						GenerateFile(
							Labels("name"),
							Expr("content", "global.unknown"),
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
					add: Doc(
						GenerateFile(
							Labels("name"),
							Expr("condition", "global.unknown"),
							Str("content", "data"),
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
					add: Doc(
						GenerateFile(
							Labels("name"),
							Str("condition", "not boolean"),
							Str("content", "data"),
						),
					),
				},
			},
			wantErr: errors.E(genfile.ErrInvalidConditionType),
		},
		{
			name:  "generate_file with lets",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/test.tm",
					add: GenerateFile(
						Labels("test"),
						Lets(
							Str("string", "let string"),
						),
						Expr("content", `let.string`),
					),
				},
			},
			want: []result{
				{
					name: "test",
					file: genFile{
						condition: true,
						body:      "let string",
					},
				},
			},
		},
		{
			name:  "generate_file with multiple lets",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/test.tm",
					add: GenerateFile(
						Labels("test"),
						Lets(
							Expr("list1", `["hello", "world"]`),
						),
						Lets(
							Expr("list2", `["lets", "feature"]`),
						),
						Expr("content", `tm_join(" ", tm_concat(let.list1, let.list2))`),
					),
				},
			},
			want: []result{
				{
					name: "test",
					file: genFile{
						condition: true,
						body:      "hello world lets feature",
					},
				},
			},
		},
		{
			name:  "using lets and metadata with interpolation",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/test.tm",
					add: GenerateFile(
						Labels("test"),
						Lets(
							Str("data", "let-data"),
						),
						Str("content", "${let.data}-${terramate.stack.path.absolute}"),
					),
				},
			},
			want: []result{
				{
					name: "test",
					file: genFile{
						condition: true,
						body:      "let-data-/stack",
					},
				},
			},
		},
		{
			name:  "using lets, globals and metadata with interpolation",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/globals.tm",
					add: Globals(
						Str("string", "global string"),
					),
				},
				{
					path: "/stack/test.tm",
					add: GenerateFile(
						Labels("test"),
						Lets(
							Expr("string", `global.string`),
							Expr("path", `terramate.stack.path.absolute`),
						),
						Str("content", "${let.string}-${let.path}"),
					),
				},
			},
			want: []result{
				{
					name: "test",
					file: genFile{
						condition: true,
						body:      "global string-/stack",
					},
				},
			},
		},
		{
			name:  "generate_file with duplicated lets attrs",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/test.tm",
					add: GenerateFile(
						Labels("test"),
						Lets(
							Str("string", "let string"),
						),
						Lets(
							Str("string", "dup"),
						),
						Expr("content", `let.string`),
					),
				},
			},
			wantErr: errors.E(lets.ErrRedefined),
		},
		{
			name:  "lets are scoped",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/test.tm",
					add: Doc(
						GenerateFile(
							Labels("test"),
							Lets(
								Str("some_str", "test"),
							),
							Expr("content", `let.some_str`),
						),
						GenerateFile(
							Labels("test2"),
							Expr("content", `let.some_str`),
						),
					),
				},
			},
			wantErr: errors.E(genfile.ErrContentEval),
		},
	}

	for _, tcase := range tcases {
		testGenfile(t, tcase)
	}
}

type (
	hclconfig struct {
		path string
		add  fmt.Stringer
	}
	genFile struct {
		origin    info.Range
		body      string
		condition bool
		asserts   []config.Assert
	}
	result struct {
		name string
		file genFile
	}
	testcase struct {
		name    string
		stack   string
		configs []hclconfig
		want    []result
		wantErr error
	}
)

func testGenfile(t *testing.T, tcase testcase) {
	t.Run(tcase.name, func(t *testing.T) {
		t.Parallel()

		s := sandbox.New(t)
		s.BuildTree([]string{"s:" + tcase.stack})
		stacks := s.LoadStacks()
		projmeta := stack.NewProjectMetadata(s.RootDir(), stacks)
		stack := s.LoadStacks()[0]

		for _, cfg := range tcase.configs {
			test.AppendFile(t, s.RootDir(), cfg.path, cfg.add.String())
		}

		cfg, err := config.LoadTree(s.RootDir(), s.RootDir())
		if errors.IsAnyKind(tcase.wantErr, hcl.ErrHCLSyntax, hcl.ErrTerramateSchema) {
			errtest.Assert(t, err, tcase.wantErr)
			return
		}

		assert.NoError(t, err)

		globals := s.LoadStackGlobals(cfg, projmeta, stack)
		vendorDir := project.NewPath("/modules")
		got, err := genfile.Load(cfg, projmeta, stack, globals, vendorDir, nil)
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
			gotfile := got[i]
			gotbody := gotfile.Body()
			wantbody := want.file.body

			if gotfile.Condition() != want.file.condition {
				t.Fatalf("got condition %t != wanted %t", gotfile.Condition(), want.file.condition)
			}

			want.file.origin = infotest.FixRange(s.RootDir(), want.file.origin)

			test.AssertEqualRanges(t, gotfile.Range(), want.file.origin, "block range")

			test.FixupRangeOnAsserts(s.RootDir(), want.file.asserts)
			test.AssertConfigEquals(t, gotfile.Asserts(), want.file.asserts)

			assert.EqualStrings(t,
				want.name,
				gotfile.Label(),
				"wrong name config path for generated code",
			)

			assert.EqualStrings(t, wantbody, gotbody,
				"generated file body differs",
			)
		}
	})
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
