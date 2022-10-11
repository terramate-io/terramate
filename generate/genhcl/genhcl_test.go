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

package genhcl_test

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/generate/genhcl"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/lets"
	"github.com/mineiros-io/terramate/stack"
	"github.com/mineiros-io/terramate/test"
	errtest "github.com/mineiros-io/terramate/test/errors"
	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
	"github.com/mineiros-io/terramate/test/sandbox"
	"github.com/rs/zerolog"
)

func TestGenerateHCL(t *testing.T) {
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
					path:     "/stack",
					filename: ".config.tm",
					add: GenerateHCL(
						Labels("config"),
						Content(
							Block("empty"),
						),
					),
				},
			},
		},
		{
			name:  "empty content block generates empty code",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("empty"),
						Content(),
					),
				},
			},
			want: []result{
				{
					name: "empty",
					hcl: genHCL{
						origin:    defaultCfg("/stack"),
						condition: true,
						body:      Doc(),
					},
				},
			},
		},
		{
			name:  "generate hcl on stack with single empty block",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("emptytest"),
						Content(
							Block("empty"),
						),
					),
				},
			},
			want: []result{
				{
					name: "emptytest",
					hcl: genHCL{
						origin:    defaultCfg("/stack"),
						condition: true,
						body:      Block("empty"),
					},
				},
			},
		},
		{
			name:  "condition set to false",
			stack: "/stack",
			configs: []hclconfig{
				{
					path:     "/stack",
					filename: "generate.tm",
					add: GenerateHCL(
						Labels("condition"),
						Bool("condition", false),
						Content(
							Block("block"),
						),
					),
				},
			},
			want: []result{
				{
					name: "condition",
					hcl: genHCL{
						origin:    "/stack/generate.tm",
						condition: false,
						body:      Doc(),
					},
				},
			},
		},
		{
			name:  "mixed conditions on different blocks",
			stack: "/stack",
			configs: []hclconfig{
				{
					path:     "/stack",
					filename: "generate.tm",
					add: Doc(
						GenerateHCL(
							Labels("condition"),
							Bool("condition", false),
							Content(
								Block("block"),
							),
						),
						GenerateHCL(
							Labels("condition2"),
							Bool("condition", true),
							Content(
								Block("block"),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "condition",
					hcl: genHCL{
						origin:    "/stack/generate.tm",
						condition: false,
						body:      Doc(),
					},
				},
				{
					name: "condition2",
					hcl: genHCL{
						origin:    "/stack/generate.tm",
						condition: true,
						body:      Block("block"),
					},
				},
			},
		},
		{
			name:  "condition evaluated from global",
			stack: "/stack",
			configs: []hclconfig{
				{
					path:     "/stack",
					filename: "globals.tm",
					add: Globals(
						Bool("condition", false),
					),
				},
				{
					path:     "/stack",
					filename: "generate.tm",
					add: GenerateHCL(
						Labels("condition"),
						Expr("condition", "global.condition"),
						Content(
							Block("block"),
						),
					),
				},
			},
			want: []result{
				{
					name: "condition",
					hcl: genHCL{
						origin:    "/stack/generate.tm",
						condition: false,
						body:      Doc(),
					},
				},
			},
		},
		{
			name:  "condition evaluated using try",
			stack: "/stack",
			configs: []hclconfig{
				{
					path:     "/stack",
					filename: "generate.tm",
					add: GenerateHCL(
						Labels("condition"),
						Expr("condition", "tm_try(global.undef, false)"),
						Content(
							Block("block"),
						),
					),
				},
			},
			want: []result{
				{
					name: "condition",
					hcl: genHCL{
						origin:    "/stack/generate.tm",
						condition: false,
						body:      Doc(),
					},
				},
			},
		},
		{
			name:  "condition evaluated using functions",
			stack: "/stack",
			configs: []hclconfig{
				{
					path:     "/stack",
					filename: "globals.tm",
					add: Globals(
						EvalExpr(t, "list", "[666]"),
					),
				},
				{
					path:     "/stack",
					filename: "generate.tm",
					add: GenerateHCL(
						Labels("condition"),
						Expr("condition", "tm_length(global.list) > 0"),
						Content(
							Block("block"),
						),
					),
				},
			},
			want: []result{
				{
					name: "condition",
					hcl: genHCL{
						origin:    "/stack/generate.tm",
						condition: true,
						body:      Block("block"),
					},
				},
			},
		},
		{
			name:  "generate hcl with only attributes on root body",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("attrs"),
						Content(
							Number("num", 666),
							Str("str", "hi"),
						),
					),
				},
			},
			want: []result{
				{
					name: "attrs",
					hcl: genHCL{
						origin:    defaultCfg("/stack"),
						condition: true,
						body: Doc(
							Number("num", 666),
							Str("str", "hi"),
						),
					},
				},
			},
		},
		{
			name:  "generate hcl with attrs referencing attrs on root",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("attrs"),
						Content(
							Number("a", 666),
							Expr("b", "a"),
						),
					),
				},
			},
			want: []result{
				{
					name: "attrs",
					hcl: genHCL{
						origin:    defaultCfg("/stack"),
						condition: true,
						body: Doc(
							Number("a", 666),
							Expr("b", "a"),
						),
					},
				},
			},
		},
		{
			name:  "generate hcl with attributes and blocks on root body",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("attrs"),
						Content(
							Number("num", 666),
							Block("test"),
							Str("str", "hi"),
						),
					),
				},
			},
			want: []result{
				{
					name: "attrs",
					hcl: genHCL{
						origin:    defaultCfg("/stack"),
						condition: true,
						body: Doc(
							Number("num", 666),
							Str("str", "hi"),
							Block("test"),
						),
					},
				},
			},
		},
		{
			name:  "scope traversals of unknown namespaces are copied as is",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("scope_traversal"),
						Content(
							Block("traversals",
								Expr("local", "local.something"),
								Expr("mul", "omg.wat.something"),
								Expr("res", "resource.something"),
								Expr("val", "omg.something"),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "scope_traversal",
					hcl: genHCL{
						origin:    defaultCfg("/stack"),
						condition: true,
						body: Block("traversals",
							Expr("local", "local.something"),
							Expr("mul", "omg.wat.something"),
							Expr("res", "resource.something"),
							Expr("val", "omg.something"),
						),
					},
				},
			},
		},
		{
			name:  "generate HCL on stack with single block",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Str("some_string", "string"),
						Number("some_number", 777),
						Bool("some_bool", true),
					),
				},
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("test"),
						Content(
							Block("testblock",
								Expr("bool", "global.some_bool"),
								Expr("number", "global.some_number"),
								Expr("string", "global.some_string"),
								Expr("obj", `{
									string = global.some_string
									number = global.some_number
									bool = global.some_bool
								}`),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "test",
					hcl: genHCL{
						origin:    defaultCfg("/stack"),
						condition: true,
						body: Block("testblock",
							Bool("bool", true),
							Number("number", 777),
							EvalExpr(t, "obj", `{
								string = "string"
								number = 777
								bool   = true
							}`),
							Str("string", "string"),
						),
					},
				},
			},
		},
		{
			name:  "generate HCL on root with multiple files",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Str("some_string", "string"),
						Number("some_number", 777),
						Bool("some_bool", true),
					),
				},
				{
					path:     "/",
					filename: "root.tm.hcl",
					add: GenerateHCL(
						Labels("test"),
						Content(
							Block("testblock",
								Expr("bool", "global.some_bool"),
								Expr("number", "global.some_number"),
								Expr("string", "global.some_string"),
							),
						),
					),
				},
				{
					path:     "/",
					filename: "root2.tm.hcl",
					add: GenerateHCL(
						Labels("test2"),
						Content(
							Block("testblock2",
								Expr("obj", `{
									string = global.some_string
									number = global.some_number
									bool = global.some_bool
								}`),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "test",
					hcl: genHCL{
						origin:    "/root.tm.hcl",
						condition: true,
						body: Block("testblock",
							Bool("bool", true),
							Number("number", 777),
							Str("string", "string"),
						),
					},
				},
				{
					name: "test2",
					hcl: genHCL{
						origin:    "/root2.tm.hcl",
						condition: true,
						body: Block("testblock2",
							EvalExpr(t, "obj", `{
								string = "string"
								number = 777
								bool   = true
							}`),
						),
					},
				},
			},
		},
		{
			name:  "generate HCL on stack with multiple files",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Str("some_string", "string"),
						Number("some_number", 777),
						Bool("some_bool", true),
					),
				},
				{
					path:     "/stack",
					filename: "test.tm.hcl",
					add: GenerateHCL(
						Labels("test"),
						Content(
							Block("testblock",
								Expr("bool", "global.some_bool"),
								Expr("number", "global.some_number"),
								Expr("string", "global.some_string"),
							),
						),
					),
				},
				{
					path:     "/stack",
					filename: "test2.tm.hcl",
					add: GenerateHCL(
						Labels("test2"),
						Content(
							Block("testblock2",
								Expr("obj", `{
									string = global.some_string
									number = global.some_number
									bool = global.some_bool
								}`),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "test",
					hcl: genHCL{
						origin:    "/stack/test.tm.hcl",
						condition: true,
						body: Block("testblock",
							Bool("bool", true),
							Number("number", 777),
							Str("string", "string"),
						),
					},
				},
				{
					name: "test2",
					hcl: genHCL{
						origin:    "/stack/test2.tm.hcl",
						condition: true,
						body: Block("testblock2",
							EvalExpr(t, "obj", `{
								string = "string"
								number = 777
								bool   = true
							}`),
						),
					},
				},
			},
		},
		{
			name:  "generate HCL on stack using try and labeled block",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						EvalExpr(t, "obj", `{
							field_a = "a"
							field_b = "b"
							field_c = "c"
						}`),
					),
				},
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("test"),
						Content(
							Block("labeled",
								Labels("label1", "label2"),
								Expr("field_a", "try(global.obj.field_a, null)"),
								Expr("field_b", "try(global.obj.field_b, null)"),
								Expr("field_c", "try(global.obj.field_c, null)"),
								Expr("field_d", "tm_try(global.obj.field_d, null)"),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "test",
					hcl: genHCL{
						origin:    defaultCfg("/stack"),
						condition: true,
						body: Block("labeled",
							Labels("label1", "label2"),
							Expr("field_a", `try("a", null)`),
							Expr("field_b", `try("b", null)`),
							Expr("field_c", `try("c", null)`),
							EvalExpr(t, "field_d", "null"),
						),
					},
				},
			},
		},
		{
			name:  "generate HCL on stack with single nested block",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Str("some_string", "string"),
						Number("some_number", 777),
						Bool("some_bool", true),
					),
				},
				{
					path:     "/stack",
					filename: "genhcl.tm.hcl",
					add: GenerateHCL(
						Labels("nesting"),
						Content(
							Block("block1",
								Expr("bool", "global.some_bool"),
								Block("block2",
									Expr("number", "global.some_number"),
									Block("block3",
										Expr("string", "global.some_string"),
										Expr("obj", `{
											string = global.some_string
											number = global.some_number
											bool = global.some_bool
										}`),
									),
								),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "nesting",
					hcl: genHCL{
						origin:    "/stack/genhcl.tm.hcl",
						condition: true,
						body: Block("block1",
							Bool("bool", true),
							Block("block2",
								Number("number", 777),
								Block("block3",
									EvalExpr(t, "obj", `{
										string = "string"
										number = 777
										bool   = true
									}`),
									Str("string", "string"),
								),
							),
						),
					},
				},
			},
		},
		{
			name:  "multiple generate HCL blocks on single file",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						Globals(
							Str("some_string", "string"),
							Number("some_number", 666),
							Bool("some_bool", true),
						),
						GenerateHCL(
							Labels("exported_one"),
							Content(
								Block("block1",
									Expr("bool", "global.some_bool"),
									Block("block2",
										Expr("number", "global.some_number"),
									),
								),
							),
						),
						GenerateHCL(
							Labels("exported_two"),
							Content(
								Block("yay",
									Expr("data", "global.some_string"),
								),
							),
						),
						GenerateHCL(
							Labels("exported_three"),
							Content(
								Block("something",
									Expr("number", "global.some_number"),
								),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "exported_one",
					hcl: genHCL{
						origin:    defaultCfg("/stack"),
						condition: true,
						body: Block("block1",
							Bool("bool", true),
							Block("block2",
								Number("number", 666),
							),
						),
					},
				},
				{
					name: "exported_three",
					hcl: genHCL{
						origin:    defaultCfg("/stack"),
						condition: true,
						body: Block("something",
							Number("number", 666),
						),
					},
				},
				{
					name: "exported_two",
					hcl: genHCL{
						origin:    defaultCfg("/stack"),
						condition: true,
						body: Block("yay",
							Str("data", "string"),
						),
					},
				},
			},
		},
		{
			name:  "generate HCL on stack parent dir",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Str("some_string", "string"),
						Number("some_number", 777),
						Bool("some_bool", true),
					),
				},
				{
					path: "/stacks",
					add: GenerateHCL(
						Labels("on_parent"),
						Content(
							Block("on_parent_block",
								Expr("obj", `{
									string = global.some_string
									number = global.some_number
									bool = global.some_bool
								}`),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "on_parent",
					hcl: genHCL{
						origin:    defaultCfg("/stacks"),
						condition: true,
						body: Block("on_parent_block",
							EvalExpr(t, "obj", `{
								string = "string"
								number = 777
								bool   = true
							}`),
						),
					},
				},
			},
		},
		{
			name:  "all metadata available by default",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/",
					add: GenerateHCL(
						Labels("root"),
						Content(
							Expr("stacks_list", "terramate.stacks.list"),
							Expr("stack_description", "terramate.stack.description"),
							Expr("stack_id", `tm_try(terramate.stack.id, "no-id")`),
							Expr("stack_name", "terramate.stack.name"),
							Expr("stack_path_abs", "terramate.stack.path.absolute"),
							Expr("stack_path_basename", "terramate.stack.path.basename"),
							Expr("stack_path_rel", "terramate.stack.path.relative"),
							Expr("stack_path_to_root", "terramate.stack.path.to_root"),
						),
					),
				},
			},
			want: []result{
				{
					name: "root",
					hcl: genHCL{
						origin:    defaultCfg("/"),
						condition: true,
						body: Doc(
							Str("stack_description", ""),
							Str("stack_id", "no-id"),
							Str("stack_name", "stack"),
							Str("stack_path_abs", "/stacks/stack"),
							Str("stack_path_basename", "stack"),
							Str("stack_path_rel", "stacks/stack"),
							Str("stack_path_to_root", "../.."),
							EvalExpr(t, "stacks_list", `["/stacks/stack"]`),
						),
					},
				},
			},
		},
		{
			name:  "stack.id metadata available",
			stack: "/stacks/stack:id=stack-id",
			configs: []hclconfig{
				{
					path: "/",
					add: GenerateHCL(
						Labels("root"),
						Content(
							Expr("stack_id", "terramate.stack.id"),
						),
					),
				},
			},
			want: []result{
				{
					name: "root",
					hcl: genHCL{
						origin:    defaultCfg("/"),
						condition: true,
						body: Doc(
							Str("stack_id", "stack-id"),
						),
					},
				},
			},
		},
		{
			name:  "generate HCL on project root dir",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/",
					add: GenerateHCL(
						Labels("root"),
						Content(
							Block("root",
								Expr("test", "terramate.stack.path.absolute"),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "root",
					hcl: genHCL{
						origin:    defaultCfg("/"),
						condition: true,
						body: Block("root",
							Str("test", "/stacks/stack"),
						),
					},
				},
			},
		},
		{
			name:  "generate HCL on all dirs of the project with different labels",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/",
					add: Doc(
						Globals(
							Str("some_string", "string"),
							Number("some_number", 777),
							Bool("some_bool", true),
						),
						GenerateHCL(
							Labels("on_root"),
							Content(
								Block("on_root_block",
									Expr("obj", `{
										string = global.some_string
									}`),
								),
							),
						),
					),
				},
				{
					path: "/stacks",
					add: GenerateHCL(
						Labels("on_parent"),
						Content(
							Block("on_parent_block",
								Expr("obj", `{
									number = global.some_number
								}`),
							),
						),
					),
				},
				{
					path: "/stacks/stack",
					add: GenerateHCL(
						Labels("on_stack"),
						Content(
							Block("on_stack_block",
								Expr("obj", `{
									bool = global.some_bool
								}`),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "on_parent",
					hcl: genHCL{
						origin:    defaultCfg("/stacks"),
						condition: true,
						body: Block("on_parent_block",
							EvalExpr(t, "obj", `{
								number = 777
							}`),
						),
					},
				},
				{
					name: "on_root",
					hcl: genHCL{
						origin:    defaultCfg("/"),
						condition: true,
						body: Block("on_root_block",
							EvalExpr(t, "obj", `{
								string = "string"
							}`),
						),
					},
				},
				{
					name: "on_stack",
					hcl: genHCL{
						origin:    defaultCfg("/stacks/stack"),
						condition: true,
						body: Block("on_stack_block",
							EvalExpr(t, "obj", `{
								bool   = true
							}`),
						),
					},
				},
			},
		},
		{
			name:  "stack with block with same label as parent is allowed",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks",
					add: GenerateHCL(
						Labels("repeated"),
						Content(
							Block("block",
								Str("data", "parent data"),
							),
						),
					),
				},
				{
					path: "/stacks/stack",
					add: GenerateHCL(
						Labels("repeated"),
						Content(
							Block("block",
								Str("data", "stack data"),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "repeated",
					hcl: genHCL{
						origin:    defaultCfg("/stacks"),
						condition: true,
						body: Block("block",
							Str("data", "parent data"),
						),
					},
				},
				{
					name: "repeated",
					hcl: genHCL{
						origin:    defaultCfg("/stacks/stack"),
						condition: true,
						body: Block("block",
							Str("data", "stack data"),
						),
					},
				},
			},
		},
		{
			name:  "stack parents with block with same label is an error",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/",
					add: GenerateHCL(
						Labels("repeated"),
						Content(
							Block("block",
								Str("data", "root data"),
							),
						),
					),
				},
				{
					path: "/stacks",
					add: GenerateHCL(
						Labels("repeated"),
						Content(
							Block("block",
								Str("data", "parent data"),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "repeated",
					hcl: genHCL{
						origin:    defaultCfg("/stacks"),
						condition: true,
						body: Block("block",
							Str("data", "parent data"),
						),
					},
				},
				{
					name: "repeated",
					hcl: genHCL{
						origin:    defaultCfg("/"),
						condition: true,
						body: Block("block",
							Str("data", "root data"),
						),
					},
				},
			},
		},
		{
			name:  "block with no label fails",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks/stack",
					add: GenerateHCL(
						Content(
							Block("block",
								Str("data", "some literal data"),
							),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name:  "generate_hcl with non-content block inside fails",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks/stack",
					add: GenerateHCL(
						Labels("test"),
						Block("block",
							Str("data", "some literal data"),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name:  "generate_hcl with other blocks than content fails",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks/stack",
					add: GenerateHCL(
						Labels("test"),
						Content(
							Str("data", "some literal data"),
						),
						Block("block",
							Str("data", "some literal data"),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name:  "generate_hcl.content block is required",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("empty"),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name:  "generate_hcl.content block with label fails",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("empty"),
						Content(
							Labels("not allowed"),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name:  "block with two labels on stack fails",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks/stack",
					add: Block("generate_hcl",
						Labels("one", "two"),
						Content(
							Block("block",
								Str("data", "some literal data"),
							),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name:  "block with empty label on stack fails",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks/stack",
					add: Block("generate_hcl",
						Labels(""),
						Content(
							Block("block",
								Str("data", "some literal data"),
							),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name:  "blocks with same label on same config is allowed",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks/stack",
					add: Doc(
						GenerateHCL(
							Labels("duplicated"),
							Content(
								Terraform(
									Str("data", "some literal data"),
								),
							),
						),
						GenerateHCL(
							Labels("duplicated"),
							Content(
								Terraform(
									Str("data2", "some literal data2"),
								),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "duplicated",
					hcl: genHCL{
						origin:    defaultCfg("/stacks/stack"),
						condition: true,
						body: Terraform(
							Str("data", "some literal data"),
						),
					},
				},
				{
					name: "duplicated",
					hcl: genHCL{
						origin:    defaultCfg("/stacks/stack"),
						condition: true,
						body: Terraform(
							Str("data2", "some literal data2"),
						),
					},
				},
			},
		},
		{
			name:  "blocks with same label on multiple config files are allowed",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path:     "/stacks/stack",
					filename: "test.tm.hcl",
					add: Doc(
						GenerateHCL(
							Labels("duplicated"),
							Content(
								Terraform(
									Str("data", "some literal data"),
								),
							),
						),
					),
				},
				{
					path:     "/stacks/stack",
					filename: "test2.tm.hcl",
					add: Doc(
						GenerateHCL(
							Labels("duplicated"),
							Content(
								Terraform(
									Str("data", "some literal data"),
								),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "duplicated",
					hcl: genHCL{
						origin:    "/stacks/stack/test.tm.hcl",
						condition: true,
						body: Terraform(
							Str("data", "some literal data"),
						),
					},
				},
				{
					name: "duplicated",
					hcl: genHCL{
						origin:    "/stacks/stack/test2.tm.hcl",
						condition: true,
						body: Terraform(
							Str("data", "some literal data"),
						),
					},
				},
			},
		},
		{
			name:  "global evaluation failure on content",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks/stack",
					add: Doc(
						GenerateHCL(
							Labels("test"),
							Content(
								Terraform(
									Expr("required_version", "global.undefined"),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrContentEval),
		},
		{
			name:  "global evaluation failure on condition",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks/stack",
					add: Doc(
						GenerateHCL(
							Labels("test"),
							Expr("condition", "global.undef"),
							Content(
								Terraform(),
							),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrConditionEval),
		},
		{
			name:  "condition attribute wont evaluate to boolean",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks/stack",
					add: Doc(
						GenerateHCL(
							Labels("test"),
							Str("condition", "not a boolean"),
							Content(
								Terraform(),
							),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrInvalidConditionType),
		},
		{
			name:  "metadata evaluation failure",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks/stack",
					add: Doc(
						GenerateHCL(
							Labels("test"),
							Content(
								Terraform(
									Expr("much_wrong", "terramate.undefined"),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrContentEval),
		},
		{
			name:  "valid config on stack but invalid on parent fails",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks",
					add: GenerateHCL(
						Block("block",
							Str("data", "some literal data"),
						),
					),
				},
				{
					path: "/stacks/stack",
					add: Doc(
						GenerateHCL(
							Labels("valid"),
							Content(
								Terraform(
									Str("data", "some literal data"),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name:  "attributes on generate_hcl block fails",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks/stack",
					add: Doc(
						GenerateHCL(
							Labels("test"),
							Str("some_attribute", "whatever"),
							Content(
								Terraform(
									Str("required_version", "1.11"),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name:  "generate HCL on stack with lets block",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("test"),
						Lets(
							Bool("some_bool", true),
							Number("some_number", 777),
							Str("some_string", "string"),
						),
						Content(
							Block("testblock",
								Expr("bool", "let.some_bool"),
								Expr("number", "let.some_number"),
								Expr("string", "let.some_string"),
								Expr("obj", `{
									string = let.some_string
									number = let.some_number
									bool = let.some_bool
								}`),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "test",
					hcl: genHCL{
						origin:    defaultCfg("/stack"),
						condition: true,
						body: Block("testblock",
							Bool("bool", true),
							Number("number", 777),
							EvalExpr(t, "obj", `{
								string = "string"
								number = 777
								bool   = true
							}`),
							Str("string", "string"),
						),
					},
				},
			},
		},
		{
			name:  "generate HCL on stack with lets referencing globals",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Str("some_string", "string"),
						Number("some_number", 777),
						Bool("some_bool", true),
					),
				},
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("test"),
						Lets(
							Expr("some_bool", `global.some_bool`),
							Expr("some_number", `global.some_number`),
							Expr("some_string", `global.some_string`),
						),
						Content(
							Block("testblock",
								Expr("bool", "let.some_bool"),
								Expr("number", "let.some_number"),
								Expr("string", "let.some_string"),
								Expr("obj", `{
									string = let.some_string
									number = let.some_number
									bool = let.some_bool
								}`),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "test",
					hcl: genHCL{
						origin:    defaultCfg("/stack"),
						condition: true,
						body: Block("testblock",
							Bool("bool", true),
							Number("number", 777),
							EvalExpr(t, "obj", `{
								string = "string"
								number = 777
								bool   = true
							}`),
							Str("string", "string"),
						),
					},
				},
			},
		},
		{
			name:  "generate HCL on stack with multiple lets block",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("test"),
						Lets(
							Bool("some_bool", true),
						),
						Lets(
							Number("some_number", 777),
						),
						Lets(
							Str("some_string", "string"),
						),
						Content(
							Block("testblock",
								Expr("bool", "let.some_bool"),
								Expr("number", "let.some_number"),
								Expr("string", "let.some_string"),
								Expr("obj", `{
									string = let.some_string
									number = let.some_number
									bool = let.some_bool
								}`),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "test",
					hcl: genHCL{
						origin:    defaultCfg("/stack"),
						condition: true,
						body: Block("testblock",
							Bool("bool", true),
							Number("number", 777),
							EvalExpr(t, "obj", `{
								string = "string"
								number = 777
								bool   = true
							}`),
							Str("string", "string"),
						),
					},
				},
			},
		},
		{
			name:  "generate HCL with duplicated lets block",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("test"),
						Lets(
							Bool("some_bool", true),
						),
						Lets(
							Bool("some_bool", false),
						),
						Content(
							Block("testblock",
								Expr("bool", "let.some_bool"),
							),
						),
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
					path: "/stack",
					add: Doc(
						GenerateHCL(
							Labels("test"),
							Lets(
								Bool("some_bool", true),
							),
							Content(
								Block("testblock",
									Expr("bool", "let.some_bool"),
								),
							),
						),
						GenerateHCL(
							Labels("test2"),
							Content(
								Block("testblock",
									Expr("bool", "let.some_bool"),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(eval.ErrPartial),
		},
	}

	for _, tcase := range tcases {
		tcase.run(t)
	}
}

type (
	hclconfig struct {
		path     string
		filename string
		add      fmt.Stringer
	}
	genHCL struct {
		body      fmt.Stringer
		origin    string
		condition bool
	}
	result struct {
		name string
		hcl  genHCL
	}
	testcase struct {
		name    string
		stack   string
		configs []hclconfig
		want    []result
		wantErr error
	}
)

func (tcase testcase) run(t *testing.T) {
	t.Run(tcase.name, func(t *testing.T) {
		s := sandbox.New(t)
		s.BuildTree([]string{"s:" + tcase.stack})
		stacks := s.LoadStacks()
		projmeta := stack.NewProjectMetadata(s.RootDir(), stacks)
		stack := stacks[0]

		for _, cfg := range tcase.configs {
			filename := cfg.filename
			if filename == "" {
				filename = config.DefaultFilename
			}
			path := filepath.Join(s.RootDir(), cfg.path)
			test.AppendFile(t, path, filename, cfg.add.String())
		}

		cfg, err := config.LoadTree(s.RootDir(), s.RootDir())
		if errors.IsAnyKind(tcase.wantErr, hcl.ErrHCLSyntax, hcl.ErrTerramateSchema) {
			errtest.Assert(t, err, tcase.wantErr)
			return
		} else {
			assert.NoError(t, err)
		}
		globals := s.LoadStackGlobals(cfg, projmeta, stack)
		got, err := genhcl.Load(cfg, projmeta, stack, globals)
		errtest.Assert(t, err, tcase.wantErr)

		if len(got) != len(tcase.want) {
			for i, file := range got {
				t.Logf("got[%d] = %v", i, file)
			}
			for i, file := range tcase.want {
				t.Logf("want[%d] = %v", i, file)
			}
			t.Fatalf("length of got and want mismatch: got %d but want %d",
				len(got), len(tcase.want))
		}

		for i, res := range tcase.want {
			gothcl := got[i]

			gotCondition := gothcl.Condition()
			wantCondition := res.hcl.condition

			if gotCondition != wantCondition {
				t.Fatalf("got condition %t != want %t", gotCondition, wantCondition)
			}

			gotcode := gothcl.Body()
			wantcode := res.hcl.body.String()

			assertHCLEquals(t, gotcode, wantcode)
			assert.EqualStrings(t,
				res.name,
				gothcl.Label(),
				"wrong name for generated code",
			)
			assert.EqualStrings(t,
				res.hcl.origin,
				gothcl.Origin().String(),
				"wrong origin config path for generated code",
			)

		}
	})
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

func defaultCfg(dir string) string {
	return path.Join(dir, config.DefaultFilename)
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
