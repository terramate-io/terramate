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
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/generate/genhcl"
	"github.com/mineiros-io/terramate/test"
	errtest "github.com/mineiros-io/terramate/test/errors"
	"github.com/mineiros-io/terramate/test/hclwrite"
	"github.com/mineiros-io/terramate/test/sandbox"
	"github.com/rs/zerolog"
)

func TestGenerateHCL(t *testing.T) {
	attr := func(name, expr string) hclwrite.BlockBuilder {
		t.Helper()
		return hclwrite.AttributeValue(t, name, expr)
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
					path:     "/stack",
					filename: ".config.tm",
					add: generateHCL(
						labels("config"),
						content(
							block("empty"),
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
					add: generateHCL(
						labels("empty"),
						content(),
					),
				},
			},
			want: []result{
				{
					name: "empty",
					hcl: genHCL{
						origin:    defaultCfg("/stack"),
						condition: true,
						body:      hcldoc(),
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
					add: generateHCL(
						labels("emptytest"),
						content(
							block("empty"),
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
						body:      block("empty"),
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
					add: generateHCL(
						labels("condition"),
						boolean("condition", false),
						content(
							block("block"),
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
						body:      hcldoc(),
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
					add: hcldoc(
						generateHCL(
							labels("condition"),
							boolean("condition", false),
							content(
								block("block"),
							),
						),
						generateHCL(
							labels("condition2"),
							boolean("condition", true),
							content(
								block("block"),
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
						body:      hcldoc(),
					},
				},
				{
					name: "condition2",
					hcl: genHCL{
						origin:    "/stack/generate.tm",
						condition: true,
						body:      block("block"),
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
					add: globals(
						boolean("condition", false),
					),
				},
				{
					path:     "/stack",
					filename: "generate.tm",
					add: generateHCL(
						labels("condition"),
						expr("condition", "global.condition"),
						content(
							block("block"),
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
						body:      hcldoc(),
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
					add: generateHCL(
						labels("condition"),
						expr("condition", "tm_try(global.undef, false)"),
						content(
							block("block"),
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
						body:      hcldoc(),
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
					add: globals(
						attr("list", "[666]"),
					),
				},
				{
					path:     "/stack",
					filename: "generate.tm",
					add: generateHCL(
						labels("condition"),
						expr("condition", "tm_length(global.list) > 0"),
						content(
							block("block"),
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
						body:      block("block"),
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
					add: generateHCL(
						labels("attrs"),
						content(
							number("num", 666),
							str("str", "hi"),
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
						body: hcldoc(
							number("num", 666),
							str("str", "hi"),
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
					add: generateHCL(
						labels("attrs"),
						content(
							number("a", 666),
							expr("b", "a"),
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
						body: hcldoc(
							number("a", 666),
							expr("b", "a"),
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
					add: generateHCL(
						labels("attrs"),
						content(
							number("num", 666),
							block("test"),
							str("str", "hi"),
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
						body: hcldoc(
							number("num", 666),
							str("str", "hi"),
							block("test"),
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
					add: generateHCL(
						labels("scope_traversal"),
						content(
							block("traversals",
								expr("local", "local.something"),
								expr("mul", "omg.wat.something"),
								expr("res", "resource.something"),
								expr("val", "omg.something"),
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
						body: block("traversals",
							expr("local", "local.something"),
							expr("mul", "omg.wat.something"),
							expr("res", "resource.something"),
							expr("val", "omg.something"),
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
					add: globals(
						str("some_string", "string"),
						number("some_number", 777),
						boolean("some_bool", true),
					),
				},
				{
					path: "/stack",
					add: generateHCL(
						labels("test"),
						content(
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
					),
				},
			},
			want: []result{
				{
					name: "test",
					hcl: genHCL{
						origin:    defaultCfg("/stack"),
						condition: true,
						body: block("testblock",
							boolean("bool", true),
							number("number", 777),
							attr("obj", `{
								string = "string"
								number = 777
								bool   = true
							}`),
							str("string", "string"),
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
					add: globals(
						str("some_string", "string"),
						number("some_number", 777),
						boolean("some_bool", true),
					),
				},
				{
					path:     "/",
					filename: "root.tm.hcl",
					add: generateHCL(
						labels("test"),
						content(
							block("testblock",
								expr("bool", "global.some_bool"),
								expr("number", "global.some_number"),
								expr("string", "global.some_string"),
							),
						),
					),
				},
				{
					path:     "/",
					filename: "root2.tm.hcl",
					add: generateHCL(
						labels("test2"),
						content(
							block("testblock2",
								expr("obj", `{
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
						body: block("testblock",
							boolean("bool", true),
							number("number", 777),
							str("string", "string"),
						),
					},
				},
				{
					name: "test2",
					hcl: genHCL{
						origin:    "/root2.tm.hcl",
						condition: true,
						body: block("testblock2",
							attr("obj", `{
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
					add: globals(
						str("some_string", "string"),
						number("some_number", 777),
						boolean("some_bool", true),
					),
				},
				{
					path:     "/stack",
					filename: "test.tm.hcl",
					add: generateHCL(
						labels("test"),
						content(
							block("testblock",
								expr("bool", "global.some_bool"),
								expr("number", "global.some_number"),
								expr("string", "global.some_string"),
							),
						),
					),
				},
				{
					path:     "/stack",
					filename: "test2.tm.hcl",
					add: generateHCL(
						labels("test2"),
						content(
							block("testblock2",
								expr("obj", `{
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
						body: block("testblock",
							boolean("bool", true),
							number("number", 777),
							str("string", "string"),
						),
					},
				},
				{
					name: "test2",
					hcl: genHCL{
						origin:    "/stack/test2.tm.hcl",
						condition: true,
						body: block("testblock2",
							attr("obj", `{
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
					add: generateHCL(
						labels("test"),
						content(
							block("labeled",
								labels("label1", "label2"),
								expr("field_a", "try(global.obj.field_a, null)"),
								expr("field_b", "try(global.obj.field_b, null)"),
								expr("field_c", "try(global.obj.field_c, null)"),
								expr("field_d", "tm_try(global.obj.field_d, null)"),
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
						body: block("labeled",
							labels("label1", "label2"),
							expr("field_a", `try("a", null)`),
							expr("field_b", `try("b", null)`),
							expr("field_c", `try("c", null)`),
							attr("field_d", "null"),
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
					add: globals(
						str("some_string", "string"),
						number("some_number", 777),
						boolean("some_bool", true),
					),
				},
				{
					path:     "/stack",
					filename: "genhcl.tm.hcl",
					add: generateHCL(
						labels("nesting"),
						content(
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
					),
				},
			},
			want: []result{
				{
					name: "nesting",
					hcl: genHCL{
						origin:    "/stack/genhcl.tm.hcl",
						condition: true,
						body: block("block1",
							boolean("bool", true),
							block("block2",
								number("number", 777),
								block("block3",
									attr("obj", `{
										string = "string"
										number = 777
										bool   = true
									}`),
									str("string", "string"),
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
					add: hcldoc(
						globals(
							str("some_string", "string"),
							number("some_number", 666),
							boolean("some_bool", true),
						),
						generateHCL(
							labels("exported_one"),
							content(
								block("block1",
									expr("bool", "global.some_bool"),
									block("block2",
										expr("number", "global.some_number"),
									),
								),
							),
						),
						generateHCL(
							labels("exported_two"),
							content(
								block("yay",
									expr("data", "global.some_string"),
								),
							),
						),
						generateHCL(
							labels("exported_three"),
							content(
								block("something",
									expr("number", "global.some_number"),
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
						body: block("block1",
							boolean("bool", true),
							block("block2",
								number("number", 666),
							),
						),
					},
				},
				{
					name: "exported_three",
					hcl: genHCL{
						origin:    defaultCfg("/stack"),
						condition: true,
						body: block("something",
							number("number", 666),
						),
					},
				},
				{
					name: "exported_two",
					hcl: genHCL{
						origin:    defaultCfg("/stack"),
						condition: true,
						body: block("yay",
							str("data", "string"),
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
					add: globals(
						str("some_string", "string"),
						number("some_number", 777),
						boolean("some_bool", true),
					),
				},
				{
					path: "/stacks",
					add: generateHCL(
						labels("on_parent"),
						content(
							block("on_parent_block",
								expr("obj", `{
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
						body: block("on_parent_block",
							attr("obj", `{
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
					add: generateHCL(
						labels("root"),
						content(
							expr("stacks_list", "terramate.stacks.list"),
							expr("stack_description", "terramate.stack.description"),
							expr("stack_id", `tm_try(terramate.stack.id, "no-id")`),
							expr("stack_name", "terramate.stack.name"),
							expr("stack_path_abs", "terramate.stack.path.absolute"),
							expr("stack_path_basename", "terramate.stack.path.basename"),
							expr("stack_path_rel", "terramate.stack.path.relative"),
							expr("stack_path_to_root", "terramate.stack.path.to_root"),
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
						body: hcldoc(
							str("stack_description", ""),
							str("stack_id", "no-id"),
							str("stack_name", "stack"),
							str("stack_path_abs", "/stacks/stack"),
							str("stack_path_basename", "stack"),
							str("stack_path_rel", "stacks/stack"),
							str("stack_path_to_root", "../.."),
							attr("stacks_list", `["/stacks/stack"]`),
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
					add: generateHCL(
						labels("root"),
						content(
							expr("stack_id", "terramate.stack.id"),
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
						body: hcldoc(
							str("stack_id", "stack-id"),
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
					add: generateHCL(
						labels("root"),
						content(
							block("root",
								expr("test", "terramate.stack.path.absolute"),
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
						body: block("root",
							str("test", "/stacks/stack"),
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
					add: hcldoc(
						globals(
							str("some_string", "string"),
							number("some_number", 777),
							boolean("some_bool", true),
						),
						generateHCL(
							labels("on_root"),
							content(
								block("on_root_block",
									expr("obj", `{
										string = global.some_string
									}`),
								),
							),
						),
					),
				},
				{
					path: "/stacks",
					add: generateHCL(
						labels("on_parent"),
						content(
							block("on_parent_block",
								expr("obj", `{
									number = global.some_number
								}`),
							),
						),
					),
				},
				{
					path: "/stacks/stack",
					add: generateHCL(
						labels("on_stack"),
						content(
							block("on_stack_block",
								expr("obj", `{
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
						body: block("on_parent_block",
							attr("obj", `{
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
						body: block("on_root_block",
							attr("obj", `{
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
						body: block("on_stack_block",
							attr("obj", `{
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
					add: generateHCL(
						labels("repeated"),
						content(
							block("block",
								str("data", "parent data"),
							),
						),
					),
				},
				{
					path: "/stacks/stack",
					add: generateHCL(
						labels("repeated"),
						content(
							block("block",
								str("data", "stack data"),
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
						body: block("block",
							str("data", "parent data"),
						),
					},
				},
				{
					name: "repeated",
					hcl: genHCL{
						origin:    defaultCfg("/stacks/stack"),
						condition: true,
						body: block("block",
							str("data", "stack data"),
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
					add: generateHCL(
						labels("repeated"),
						content(
							block("block",
								str("data", "root data"),
							),
						),
					),
				},
				{
					path: "/stacks",
					add: generateHCL(
						labels("repeated"),
						content(
							block("block",
								str("data", "parent data"),
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
						body: block("block",
							str("data", "parent data"),
						),
					},
				},
				{
					name: "repeated",
					hcl: genHCL{
						origin:    defaultCfg("/"),
						condition: true,
						body: block("block",
							str("data", "root data"),
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
					add: generateHCL(
						content(
							block("block",
								str("data", "some literal data"),
							),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrParsing),
		},
		{
			name:  "generate_hcl with non-content block inside fails",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks/stack",
					add: generateHCL(
						labels("test"),
						block("block",
							str("data", "some literal data"),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrParsing),
		},
		{
			name:  "generate_hcl with other blocks than content fails",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks/stack",
					add: generateHCL(
						labels("test"),
						content(
							str("data", "some literal data"),
						),
						block("block",
							str("data", "some literal data"),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrParsing),
		},
		{
			name:  "generate_hcl.content block is required",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("empty"),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrParsing),
		},
		{
			name:  "generate_hcl.content block with label fails",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("empty"),
						content(
							labels("not allowed"),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrParsing),
		},
		{
			name:  "block with two labels on stack fails",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks/stack",
					add: block("generate_hcl",
						labels("one", "two"),
						content(
							block("block",
								str("data", "some literal data"),
							),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrParsing),
		},
		{
			name:  "block with empty label on stack fails",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks/stack",
					add: block("generate_hcl",
						labels(""),
						content(
							block("block",
								str("data", "some literal data"),
							),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrParsing),
		},
		{
			name:  "blocks with same label on same config is allowed",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks/stack",
					add: hcldoc(
						generateHCL(
							labels("duplicated"),
							content(
								terraform(
									str("data", "some literal data"),
								),
							),
						),
						generateHCL(
							labels("duplicated"),
							content(
								terraform(
									str("data2", "some literal data2"),
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
						body: terraform(
							str("data", "some literal data"),
						),
					},
				},
				{
					name: "duplicated",
					hcl: genHCL{
						origin:    defaultCfg("/stacks/stack"),
						condition: true,
						body: terraform(
							str("data2", "some literal data2"),
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
					add: hcldoc(
						generateHCL(
							labels("duplicated"),
							content(
								terraform(
									str("data", "some literal data"),
								),
							),
						),
					),
				},
				{
					path:     "/stacks/stack",
					filename: "test2.tm.hcl",
					add: hcldoc(
						generateHCL(
							labels("duplicated"),
							content(
								terraform(
									str("data", "some literal data"),
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
						body: terraform(
							str("data", "some literal data"),
						),
					},
				},
				{
					name: "duplicated",
					hcl: genHCL{
						origin:    "/stacks/stack/test2.tm.hcl",
						condition: true,
						body: terraform(
							str("data", "some literal data"),
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
					add: hcldoc(
						generateHCL(
							labels("test"),
							content(
								terraform(
									expr("required_version", "global.undefined"),
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
					add: hcldoc(
						generateHCL(
							labels("test"),
							expr("condition", "global.undef"),
							content(
								terraform(),
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
					add: hcldoc(
						generateHCL(
							labels("test"),
							str("condition", "not a boolean"),
							content(
								terraform(),
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
					add: hcldoc(
						generateHCL(
							labels("test"),
							content(
								terraform(
									expr("much_wrong", "terramate.undefined"),
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
					add: generateHCL(
						block("block",
							str("data", "some literal data"),
						),
					),
				},
				{
					path: "/stacks/stack",
					add: hcldoc(
						generateHCL(
							labels("valid"),
							content(
								terraform(
									str("data", "some literal data"),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrParsing),
		},
		{
			name:  "attributes on generate_hcl block fails",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks/stack",
					add: hcldoc(
						generateHCL(
							labels("test"),
							str("some_attribute", "whatever"),
							content(
								terraform(
									str("required_version", "1.11"),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrParsing),
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
		stack := s.LoadStacks()[0]

		for _, cfg := range tcase.configs {
			filename := cfg.filename
			if filename == "" {
				filename = config.DefaultFilename
			}
			path := filepath.Join(s.RootDir(), cfg.path)
			test.AppendFile(t, path, filename, cfg.add.String())
		}

		globals := s.LoadStackGlobals(stack)
		got, err := genhcl.Load(s.RootDir(), stack, globals)
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
				gothcl.Name(),
				"wrong name for generated code",
			)
			assert.EqualStrings(t,
				res.hcl.origin,
				gothcl.Origin(),
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
	return filepath.Join(dir, config.DefaultFilename)
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
