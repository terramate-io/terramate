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
	"testing"

	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
)

func TestGenerateHCLDynamic(t *testing.T) {
	tcases := []testcase{
		{
			name:  "tm_dynamic with empty content block",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("tm_dynamic_test.tf"),
						content(
							tmdynamic(
								labels("my_block"),
								expr("for_each", `["a", "b", "c"]`),
								block("content"),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "tm_dynamic_test.tf",
					hcl: genHCL{
						origin:    defaultCfg("/stack"),
						condition: true,
						body: hcldoc(
							block("my_block"),
							block("my_block"),
							block("my_block"),
						),
					},
				},
			},
		},
		{
			name:  "tm_dynamic with content fully evaluated",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("tm_dynamic_test.tf"),
						content(
							tmdynamic(
								labels("my_block"),
								expr("for_each", `["a", "b", "c"]`),
								content(
									expr("value", "my_block.value"),
									expr("key", "my_block.key"),
								),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "tm_dynamic_test.tf",
					hcl: genHCL{
						origin:    defaultCfg("/stack"),
						condition: true,
						body: hcldoc(
							block("my_block",
								number("key", 0),
								str("value", "a"),
							),
							block("my_block",
								number("key", 1),
								str("value", "b"),
							),
							block("my_block",
								number("key", 2),
								str("value", "c"),
							),
						),
					},
				},
			},
		},
		{
			name:  "tm_dynamic with labels from globals",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: hcldoc(
						globals(
							expr("labels", `["label1", "label2"]`),
						),
						generateHCL(
							labels("tm_dynamic_test.tf"),
							content(
								tmdynamic(
									labels("my_block"),
									expr("for_each", `["a", "b", "c"]`),
									expr("labels", `global.labels`),
									content(
										expr("value", "my_block.value"),
										expr("key", "my_block.key"),
									),
								),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "tm_dynamic_test.tf",
					hcl: genHCL{
						origin:    defaultCfg("/stack"),
						condition: true,
						body: hcldoc(
							block("my_block",
								labels("label1", "label2"),
								number("key", 0),
								str("value", "a"),
							),
							block("my_block",
								labels("label1", "label2"),
								number("key", 1),
								str("value", "b"),
							),
							block("my_block",
								labels("label1", "label2"),
								number("key", 2),
								str("value", "c"),
							),
						),
					},
				},
			},
		},
		{
			name:  "tm_dynamic with labels from iterator variable",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: hcldoc(
						generateHCL(
							labels("tm_dynamic_test.tf"),
							content(
								tmdynamic(
									labels("my_block"),
									expr("for_each", `["a", "b", "c"]`),
									expr("labels", `[my_block.value]`),
									content(
										expr("value", "my_block.value"),
										expr("key", "my_block.key"),
									),
								),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "tm_dynamic_test.tf",
					hcl: genHCL{
						origin:    defaultCfg("/stack"),
						condition: true,
						body: hcldoc(
							block("my_block",
								labels("a"),
								number("key", 0),
								str("value", "a"),
							),
							block("my_block",
								labels("b"),
								number("key", 1),
								str("value", "b"),
							),
							block("my_block",
								labels("c"),
								number("key", 2),
								str("value", "c"),
							),
						),
					},
				},
			},
		},
		{
			name:  "tm_dynamic partially evaluated",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("tm_dynamic_test.tf"),
						content(
							tmdynamic(
								labels("my_block"),
								expr("for_each", `["a", "b", "c"]`),
								content(
									expr("value", "my_block.value"),
									expr("key", "my_block.key"),
									expr("other", "something.other"),
								),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "tm_dynamic_test.tf",
					hcl: genHCL{
						origin:    defaultCfg("/stack"),
						condition: true,
						body: hcldoc(
							block("my_block",
								number("key", 0),
								expr("other", "something.other"),
								str("value", "a"),
							),
							block("my_block",
								number("key", 1),
								expr("other", "something.other"),
								str("value", "b"),
							),
							block("my_block",
								number("key", 2),
								expr("other", "something.other"),
								str("value", "c"),
							),
						),
					},
				},
			},
		},
		{
			name:  "tm_dynamic with different iterator",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("tm_dynamic_test.tf"),
						content(
							tmdynamic(
								labels("my_block"),
								expr("for_each", `["a", "b", "c"]`),
								expr("iterator", "b"),
								content(
									expr("value", "b.value"),
									expr("key", "b.key"),
									expr("other", "something.other"),
								),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "tm_dynamic_test.tf",
					hcl: genHCL{
						origin:    defaultCfg("/stack"),
						condition: true,
						body: hcldoc(
							block("my_block",
								number("key", 0),
								expr("other", "something.other"),
								str("value", "a"),
							),
							block("my_block",
								number("key", 1),
								expr("other", "something.other"),
								str("value", "b"),
							),
							block("my_block",
								number("key", 2),
								expr("other", "something.other"),
								str("value", "c"),
							),
						),
					},
				},
			},
		},
		{
			name:  "empty attributes generates empty blocks",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("empty.tf"),
						content(
							tmdynamic(
								labels("empty"),
								expr("for_each", `["a", "b"]`),
								expr("attributes", `{}`),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "empty.tf",
					hcl: genHCL{
						origin:    defaultCfg("/stack"),
						condition: true,
						body: hcldoc(
							block("empty"),
							block("empty"),
						),
					},
				},
			},
		},
		{
			name:  "using partially evaluated attributes",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("tm_dynamic_test.tf"),
						content(
							tmdynamic(
								labels("attributes"),
								expr("for_each", `["a", "b", "c"]`),
								expr("iterator", "iter"),
								expr("attributes", `{
								  value = iter.value,
								  key = iter.key,
								  other = something.other,
								}`),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "tm_dynamic_test.tf",
					hcl: genHCL{
						origin:    defaultCfg("/stack"),
						condition: true,
						body: hcldoc(
							block("attributes",
								str("value", "a"),
								number("key", 0),
								expr("other", "something.other"),
							),
							block("attributes",
								str("value", "b"),
								number("key", 1),
								expr("other", "something.other"),
							),
							block("attributes",
								str("value", "c"),
								number("key", 2),
								expr("other", "something.other"),
							),
						),
					},
				},
			},
		},
		{
			name:  "attributes is result of tm function",
			stack: "/stack",
			configs: []hclconfig{
				{
					path:     "/stack",
					filename: "globals.tm",
					add: globals(
						expr("obj", `{
						  a = "global data"
						}`),
					),
				},
				{
					path:     "/stack",
					filename: "gen.tm",
					add: generateHCL(
						labels("test.tf"),
						content(
							tmdynamic(
								labels("test"),
								expr("for_each", `["a"]`),
								expr("iterator", "iter"),
								expr("attributes", `tm_merge(global.obj, {
								  b = 666,
								})`),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "test.tf",
					hcl: genHCL{
						origin:    "/stack/gen.tm",
						condition: true,
						body: hcldoc(
							block("test",
								str("a", "global data"),
								number("b", 666),
							),
						),
					},
				},
			},
		},
		{
			name:  "generated blocks have attributes on same order as attributes object",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("tm_dynamic_attributes_order.tf"),
						content(
							tmdynamic(
								labels("attributes_order"),
								expr("for_each", `["test", "test2"]`),
								expr("iterator", "iter"),
								expr("attributes", `{
								  b = iter.value,
								  z = iter.key,
								  a = something.other,
								}`),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "tm_dynamic_attributes_order.tf",
					hcl: genHCL{
						origin:    defaultCfg("/stack"),
						condition: true,
						body: hcldoc(
							block("attributes_order",
								str("b", "test"),
								number("z", 0),
								expr("a", "something.other"),
							),
							block("attributes_order",
								str("b", "test2"),
								number("z", 1),
								expr("a", "something.other"),
							),
						),
					},
				},
			},
		},
		{
			name:  "attributes referencing globals and metadata with functions",
			stack: "/stack",
			configs: []hclconfig{
				{
					path:     "/",
					filename: "globals.tm",
					add: globals(
						str("data", "global string"),
					),
				},
				{
					path:     "/stack",
					filename: "generate.tm",
					add: generateHCL(
						labels("tm_dynamic.tf"),
						content(
							tmdynamic(
								labels("references"),
								expr("for_each", `["test"]`),
								expr("attributes", `{
								  global  = global.data,
								  meta    = terramate.stack.path.absolute,
								  interp  = tm_upper("${global.data} interp"),
								  partial = local.data,
								  iter    = references.value
								  partialfunc = upper(global.data)
								  ternary = local.cond ? local.val : global.data
								}`),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "tm_dynamic.tf",
					hcl: genHCL{
						origin:    "/stack/generate.tm",
						condition: true,
						body: hcldoc(
							block("references",
								str("global", "global string"),
								str("meta", "/stack"),
								str("interp", "GLOBAL STRING INTERP"),
								expr("partial", "local.data"),
								str("iter", "test"),
								expr("partialfunc", `upper("global string")`),
								expr("ternary", `local.cond ? local.val : "global string"`),
							),
						),
					},
				},
			},
		},
		{
			name:  "attributes keys referencing globals and metadata with functions",
			stack: "/stack",
			configs: []hclconfig{
				{
					path:     "/",
					filename: "globals.tm",
					add: globals(
						str("key", "globalkey"),
					),
				},
				{
					path:     "/stack",
					filename: "generate.tm",
					add: generateHCL(
						labels("tm_dynamic.tf"),
						content(
							tmdynamic(
								labels("references"),
								expr("for_each", `["test"]`),
								expr("attributes", `{
								  (global.key) = true,
								  (terramate.stack.name) = true,
								  (tm_upper(global.key)) = true,
								  (references.value) = true,
								}`),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "tm_dynamic.tf",
					hcl: genHCL{
						origin:    "/stack/generate.tm",
						condition: true,
						body: hcldoc(
							block("references",
								boolean("globalkey", true),
								boolean("stack", true),
								boolean("GLOBALKEY", true),
								boolean("test", true),
							),
						),
					},
				},
			},
		},
		{
			name:  "tm_dynamic inside tm_dynamic using attributes",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("tm_dynamic_test.tf"),
						content(
							tmdynamic(
								labels("parent"),
								expr("for_each", `["a", "b"]`),
								content(
									expr("value", "parent.value"),
									expr("key", "parent.key"),
									expr("other", "something.other"),
									tmdynamic(
										labels("child"),
										expr("for_each", `[0, 1]`),
										expr("attributes", `{
											value = "${parent.key}-${parent.value}-${child.value}",
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
					name: "tm_dynamic_test.tf",
					hcl: genHCL{
						origin:    defaultCfg("/stack"),
						condition: true,
						body: hcldoc(
							block("parent",
								number("key", 0),
								expr("other", "something.other"),
								str("value", "a"),
								block("child",
									str("value", "0-a-0"),
								),
								block("child",
									str("value", "0-a-1"),
								),
							),
							block("parent",
								number("key", 1),
								expr("other", "something.other"),
								str("value", "b"),
								block("child",
									str("value", "1-b-0"),
								),
								block("child",
									str("value", "1-b-1"),
								),
							),
						),
					},
				},
			},
		},
		{
			name:  "tm_dynamic inside tm_dynamic using content",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("tm_dynamic_test.tf"),
						content(
							tmdynamic(
								labels("my_block"),
								expr("for_each", `["a", "b", "c"]`),
								expr("iterator", "b"),
								content(
									expr("value", "b.value"),
									expr("key", "b.key"),
									expr("other", "something.other"),
									tmdynamic(
										labels("child"),
										expr("for_each", `[0, 1, 2]`),
										expr("iterator", "i"),
										content(
											str("value", "${b.key}-${b.value}-${i.value}"),
										),
									),
								),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "tm_dynamic_test.tf",
					hcl: genHCL{
						origin:    defaultCfg("/stack"),
						condition: true,
						body: hcldoc(
							block("my_block",
								number("key", 0),
								expr("other", "something.other"),
								str("value", "a"),
								block("child",
									str("value", "0-a-0"),
								),
								block("child",
									str("value", "0-a-1"),
								),
								block("child",
									str("value", "0-a-2"),
								),
							),
							block("my_block",
								number("key", 1),
								expr("other", "something.other"),
								str("value", "b"),
								block("child",
									str("value", "1-b-0"),
								),
								block("child",
									str("value", "1-b-1"),
								),
								block("child",
									str("value", "1-b-2"),
								),
							),
							block("my_block",
								number("key", 2),
								expr("other", "something.other"),
								str("value", "c"),
								block("child",
									str("value", "2-c-0"),
								),
								block("child",
									str("value", "2-c-1"),
								),
								block("child",
									str("value", "2-c-2"),
								),
							),
						),
					},
				},
			},
		},
		{
			name:  "tm_dynamic using globals in for_each and content",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: hcldoc(
						globals(
							str("msg", "hello"),
							expr("values", `["a", "b", "c"]`),
						),
						generateHCL(
							labels("tm_dynamic_test.tf"),
							content(
								tmdynamic(
									labels("my_block"),
									expr("for_each", `global.values`),
									content(
										expr("msg", `global.msg`),
										expr("val", `global.values[my_block.key]`),
									),
								),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "tm_dynamic_test.tf",
					hcl: genHCL{
						origin:    defaultCfg("/stack"),
						condition: true,
						body: hcldoc(
							block("my_block",
								str("msg", "hello"),
								str("val", "a"),
							),
							block("my_block",
								str("msg", "hello"),
								str("val", "b"),
							),
							block("my_block",
								str("msg", "hello"),
								str("val", "c"),
							),
						),
					},
				},
			},
		},
		{
			name:  "tm_dynamic with invalid iterator",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("tm_dynamic_test.tf"),
						content(
							tmdynamic(
								labels("my_block"),
								expr("for_each", `["a", "b", "c"]`),
								expr("iterator", "[]"),
								content(
									expr("value", "b.value"),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrInvalidDynamicIterator),
		},
		{
			name:  "no content block and no attributes fails",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("tm_dynamic_test.tf"),
						content(
							tmdynamic(
								labels("my_block"),
								expr("for_each", `["a", "b", "c"]`),
							),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name:  "attributes is null fails",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("tm_dynamic_test.tf"),
						content(
							tmdynamic(
								labels("my_block"),
								expr("for_each", `["a"]`),
								expr("attributes", "null"),
							),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name:  "attributes is not object fails",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("tm_dynamic_test.tf"),
						content(
							tmdynamic(
								labels("my_block"),
								expr("for_each", `["a"]`),
								expr("attributes", "[]"),
							),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name:  "attributes key is undefined fails",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("fail.tf"),
						content(
							tmdynamic(
								labels("my_block"),
								expr("for_each", `["a"]`),
								expr("attributes", `{ (local.a) : 666 }`),
							),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name:  "attributes key is not a string fails",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: hcldoc(
						globals(
							number("a", 666),
						),
						generateHCL(
							labels("fail.tf"),
							content(
								tmdynamic(
									labels("my_block"),
									expr("for_each", `["a"]`),
									expr("attributes", `{ (global.a) : 666 }`),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name:  "attributes and unknown attribute fails",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("tm_dynamic_test.tf"),
						content(
							tmdynamic(
								labels("my_block"),
								expr("for_each", `["a", "b", "c"]`),
								expr("attributes", `{ b : 666 }`),
								str("something", "val"),
							),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name:  "tm_dynamic with more than one label fails",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("tm_dynamic_test.tf"),
						content(
							tmdynamic(
								labels("my_block", "nope"),
								expr("for_each", `["a"]`),
								content(
									str("a", "val"),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name:  "tm_dynamic with multiple content blocks fail",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("tm_dynamic_test.tf"),
						content(
							tmdynamic(
								labels("my_block"),
								expr("for_each", `["a"]`),
								content(
									str("a", "val"),
								),
								content(
									str("a", "val"),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name:  "tm_dynamic with unknown block fail",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("tm_dynamic_test.tf"),
						content(
							tmdynamic(
								labels("my_block"),
								expr("for_each", `["a"]`),
								content(
									str("a", "val"),
								),
								block("unsupported",
									str("a", "val"),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name:  "tm_dynamic with undefined for_each fail",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("tm_dynamic_test.tf"),
						content(
							tmdynamic(
								labels("my_block"),
								expr("for_each", `[local.a]`),
								content(
									str("a", "val"),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name:  "tm_dynamic with labels with undefined references fails",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("tm_dynamic_test.tf"),
						content(
							tmdynamic(
								labels("my_block"),
								expr("for_each", `["a"]`),
								expr("labels", "[local.a]"),
								content(
									str("a", "val"),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name:  "tm_dynamic with labels that is not a list fails",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("tm_dynamic_test.tf"),
						content(
							tmdynamic(
								labels("my_block"),
								expr("for_each", `["a"]`),
								expr("labels", "{}"),
								content(
									str("a", "val"),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name:  "tm_dynamic with iterator with traversal fails",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("tm_dynamic_test.tf"),
						content(
							tmdynamic(
								labels("my_block"),
								expr("for_each", `["a"]`),
								expr("iterator", "name.traverse"),
								content(
									str("a", "val"),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name:  "tm_dynamic with non-iterable for_each fail",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("tm_dynamic_test.tf"),
						content(
							tmdynamic(
								labels("my_block"),
								number("for_each", 666),
								content(
									str("a", "val"),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name:  "tm_dynamic with no label fails",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("tm_dynamic_test.tf"),
						content(
							tmdynamic(
								expr("for_each", `["a"]`),
								content(
									str("a", "val"),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name:  "content and unknown attribute fails",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("tm_dynamic_test.tf"),
						content(
							tmdynamic(
								labels("my_block"),
								expr("for_each", `["a", "b", "c"]`),
								str("something", "val"),
								content(
									str("a", "val"),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name:  "content block and attributes fails",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("tm_dynamic_test.tf"),
						content(
							tmdynamic(
								labels("my_block"),
								expr("for_each", `["a", "b", "c"]`),
								expr("attributes", `{ b : 666 }`),
								content(
									str("a", "val"),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name:  "tm_dynamic with no for_each fails",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("tm_dynamic_test.tf"),
						content(
							tmdynamic(
								labels("my_block"),
								content(
									expr("value", "b.value"),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
	}

	for _, tcase := range tcases {
		tcase.run(t)
	}
}
