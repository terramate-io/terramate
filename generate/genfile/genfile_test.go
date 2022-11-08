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
	"github.com/mineiros-io/terramate/generate/genfile"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/hcl/info"
	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/stack"
	"github.com/mineiros-io/terramate/test"
	errtest "github.com/mineiros-io/terramate/test/errors"
	infotest "github.com/mineiros-io/terramate/test/hclutils/info"
	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
	"github.com/mineiros-io/terramate/test/sandbox"
	"github.com/rs/zerolog"
)

func TestLoadGenerateFilesForStackContext(t *testing.T) {
	t.Parallel()

	tcases := []testcase{
		{
			name:    "empty content attribute generates empty body",
			context: "both",
			block: GenerateFile(
				Labels("empty"),
				Str("content", ""),
			),
			want: genFile{
				body:      "",
				condition: true,
			},
		},
		{
			name:    "simple plain string",
			context: "both",
			block: GenerateFile(
				Labels("test"),
				Str("content", "test"),
			),
			want: genFile{
				body:      "test",
				condition: true,
			},
		},
		{
			name:    "all metadata available by default for stack",
			context: "stack",
			block: GenerateFile(
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
			want: genFile{
				condition: true,
				body: `
stacks_list=["/"]
stack_path_abs=/
stack_path_rel=.
stack_path_to_root=.
stack_path_basename=.
stack_id=no-id
stack_name=stack
stack_description=
`,
			},
		},
		/*{
					name:   "stack.id metadata available",
					layout: []string{"s:stack:id=stack-id"},
					dir:    "/stack",
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
					name:   "using globals and metadata with interpolation",
					layout: []string{"s:stack"},
					dir:    "/stack",
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
					name:   "condition set to false",
					layout: []string{"s:stack"},
					dir:    "/stack",
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
					name:   "mixed conditions on different blocks",
					layout: []string{"s:stack"},
					dir:    "/stack",
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
					name:   "condition evaluated from global",
					layout: []string{"s:stack"},
					dir:    "/stack",
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
					name:   "condition evaluated using try",
					layout: []string{"s:stack"},
					dir:    "/stack",
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
					name:   "condition evaluated using functions",
					layout: []string{"s:stack"},
					dir:    "/stack",
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
					name:   "multiple generate_file blocks on same file",
					layout: []string{"s:stack"},
					dir:    "/stack",
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
					name:   "using globals and metadata with functions",
					layout: []string{"s:stack"},
					dir:    "/stack",
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
					name:   "hierarchical load",
					layout: []string{"s:stacks/stack"},
					dir:    "/stacks/stack",
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
					name:   "content must be string",
					layout: []string{"s:stack"},
					dir:    "/stack",
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
					name:   "blocks with same label are allowed",
					layout: []string{"s:stack"},
					dir:    "/stack",
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
					name:   "same labels but different condition",
					layout: []string{"s:stack"},
					dir:    "/stack",
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
					name:   "conflicting blocks on same dir",
					layout: []string{"s:stack"},
					dir:    "/stack",
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
					name:   "conflicting blocks on different dirs",
					layout: []string{"s:stack"},
					dir:    "/stack",
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
					name:   "generate_file missing label",
					layout: []string{"s:stack"},
					dir:    "/stack",
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
					name:   "generate_file with two labels",
					layout: []string{"s:stack"},
					dir:    "/stack",
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
					name:   "generate_file with empty label",
					layout: []string{"s:stack"},
					dir:    "/stack",
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
					name:   "generate_file missing content attribute",
					layout: []string{"s:stack"},
					dir:    "/stack",
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
					name:   "generate_file with unknown attribute",
					layout: []string{"s:stack"},
					dir:    "/stack",
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
					name:   "generate_file fails to evaluate content",
					layout: []string{"s:stack"},
					dir:    "/stack",
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
					name:   "generate_file fails to evaluate condition",
					layout: []string{"s:stack"},
					dir:    "/stack",
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
					name:   "generate_file fails condition dont evaluate to boolean",
					layout: []string{"s:stack"},
					dir:    "/stack",
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
					name:   "generate_file with lets",
					layout: []string{"s:stack"},
					dir:    "/stack",
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
					name:   "generate_file with multiple lets",
					layout: []string{"s:stack"},
					dir:    "/stack",
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
					name:   "using lets and metadata with interpolation",
					layout: []string{"s:stack"},
					dir:    "/stack",
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
					name:   "using lets, globals and metadata with interpolation",
					layout: []string{"s:stack"},
					dir:    "/stack",
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
					name:   "generate_file with duplicated lets attrs",
					layout: []string{"s:stack"},
					dir:    "/stack",
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
					name:   "lets are scoped",
					layout: []string{"s:stack"},
					dir:    "/stack",
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
		*/
	}

	for _, tcase := range tcases {
		testGenfile(t, tcase)
	}
}

type (
	genFile struct {
		origin    info.Range
		body      string
		condition bool
		asserts   []config.Assert
	}
	testcase struct {
		name    string
		context string
		globals fmt.Stringer
		block   fmt.Stringer
		want    genFile
		wantErr error
	}
)

func testGenfile(t *testing.T, tcase testcase) {
	var contextCases []string
	switch tcase.context {
	case "both":
		contextCases = []string{"stack", "root"}
	case "stack":
		contextCases = []string{"stack"}
	case "root":
		contextCases = []string{"root"}
	}

	for _, context := range contextCases {
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			s := sandbox.New(t)
			if context == "stack" {
				assert.NoError(t, stack.Create(s.Config(), stack.CreateCfg{
					Dir:  s.RootDir(),
					Name: "root stack",
				}))
			}

			rootcfg := s.ReloadConfig()
			stacks, err := stack.LoadAll(rootcfg.Tree)
			assert.NoError(t, err)
			stackpaths := make(project.Paths, len(stacks))
			for i, st := range stacks {
				stackpaths[i] = st.Path()
			}

			var ctx *eval.Context
			projmeta := project.NewMetadata(s.RootDir(), stackpaths)
			if context == "stack" {
				st, err := stack.New(rootcfg.RootDir(), rootcfg.Tree.Node)
				assert.NoError(t, err)

				ctx, _ = stack.LoadStackGlobals(rootcfg, projmeta, st)
			} else {
				ctx, err = eval.NewContext(s.RootDir())
				assert.NoError(t, err)
				ctx.SetNamespace("terramate", projmeta.ToCtyMap())
			}

			test.AppendFile(t, s.RootDir(), "block.tm", tcase.block.String())
			if tcase.globals != nil {
				test.AppendFile(t, s.RootDir(), "globals.tm", tcase.globals.String())
			}

			blocks := rootcfg.Tree.DownwardGenerateFiles()
			assert.EqualInts(t, 1, len(blocks), "expected just 1 block from tcase")
			block := blocks[0]
			got, err := genfile.Eval(block, ctx)
			errtest.Assert(t, err, tcase.wantErr)

			gotbody := got.Body()
			wantbody := tcase.want.body

			if got.Condition() != tcase.want.condition {
				t.Fatalf("got condition %t != wanted %t", got.Condition(),
					tcase.want.condition)
			}

			if got.Context() != context {
				t.Fatalf("got unexpected context %s but expected %s",
					got.Context(), context)
			}

			tcase.want.origin = infotest.FixRange(s.RootDir(), tcase.want.origin)

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

		})
	}
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
