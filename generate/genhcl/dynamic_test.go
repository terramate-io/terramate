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
	"github.com/mineiros-io/terramate/generate/genhcl"
	"github.com/mineiros-io/terramate/hcl/eval"
	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
)

func TestGenerateHCLDynamic(t *testing.T) {
	t.Parallel()

	tcases := []testcase{
		{
			name:  "tm_dynamic with empty content block",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("tm_dynamic_test.tf"),
						Content(
							TmDynamic(
								Labels("my_block"),
								Expr("for_each", `["a", "b", "c"]`),
								Block("content"),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "tm_dynamic_test.tf",
					hcl: genHCL{
						condition: true,
						body: Doc(
							Block("my_block"),
							Block("my_block"),
							Block("my_block"),
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
					add: GenerateHCL(
						Labels("tm_dynamic_test.tf"),
						Content(
							TmDynamic(
								Labels("my_block"),
								Expr("for_each", `["a", "b", "c"]`),
								Content(
									Expr("value", "my_block.value"),
									Expr("key", "my_block.key"),
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
						condition: true,
						body: Doc(
							Block("my_block",
								Number("key", 0),
								Str("value", "a"),
							),
							Block("my_block",
								Number("key", 1),
								Str("value", "b"),
							),
							Block("my_block",
								Number("key", 2),
								Str("value", "c"),
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
					add: Doc(
						Globals(
							Expr("labels", `["label1", "label2"]`),
						),
						GenerateHCL(
							Labels("tm_dynamic_test.tf"),
							Content(
								TmDynamic(
									Labels("my_block"),
									Expr("for_each", `["a", "b", "c"]`),
									Expr("labels", `global.labels`),
									Content(
										Expr("value", "my_block.value"),
										Expr("key", "my_block.key"),
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
						condition: true,
						body: Doc(
							Block("my_block",
								Labels("label1", "label2"),
								Number("key", 0),
								Str("value", "a"),
							),
							Block("my_block",
								Labels("label1", "label2"),
								Number("key", 1),
								Str("value", "b"),
							),
							Block("my_block",
								Labels("label1", "label2"),
								Number("key", 2),
								Str("value", "c"),
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
					add: Doc(
						GenerateHCL(
							Labels("tm_dynamic_test.tf"),
							Content(
								TmDynamic(
									Labels("my_block"),
									Expr("for_each", `["a", "b", "c"]`),
									Expr("labels", `[my_block.value]`),
									Content(
										Expr("value", "my_block.value"),
										Expr("key", "my_block.key"),
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
						condition: true,
						body: Doc(
							Block("my_block",
								Labels("a"),
								Number("key", 0),
								Str("value", "a"),
							),
							Block("my_block",
								Labels("b"),
								Number("key", 1),
								Str("value", "b"),
							),
							Block("my_block",
								Labels("c"),
								Number("key", 2),
								Str("value", "c"),
							),
						),
					},
				},
			},
		},
		{
			name:  "tm_dynamic with duplicated labels",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						GenerateHCL(
							Labels("tm_dynamic_test.tf"),
							Content(
								TmDynamic(
									Labels("duplicated_labels"),
									Expr("for_each", `["val"]`),
									Expr("labels", `["a", "a"]`),
									Content(
										Str("value", "str"),
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
						condition: true,
						body: Doc(
							Block("duplicated_labels",
								Labels("a", "a"),
								Str("value", "str"),
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
					add: GenerateHCL(
						Labels("tm_dynamic_test.tf"),
						Content(
							TmDynamic(
								Labels("my_block"),
								Expr("for_each", `["a", "b", "c"]`),
								Content(
									Expr("value", "my_block.value"),
									Expr("key", "my_block.key"),
									Expr("other", "something.other"),
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
						condition: true,
						body: Doc(
							Block("my_block",
								Number("key", 0),
								Expr("other", "something.other"),
								Str("value", "a"),
							),
							Block("my_block",
								Number("key", 1),
								Expr("other", "something.other"),
								Str("value", "b"),
							),
							Block("my_block",
								Number("key", 2),
								Expr("other", "something.other"),
								Str("value", "c"),
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
					add: GenerateHCL(
						Labels("tm_dynamic_test.tf"),
						Content(
							TmDynamic(
								Labels("my_block"),
								Expr("for_each", `["a", "b", "c"]`),
								Expr("iterator", "b"),
								Content(
									Expr("value", "b.value"),
									Expr("key", "b.key"),
									Expr("other", "something.other"),
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
						condition: true,
						body: Doc(
							Block("my_block",
								Number("key", 0),
								Expr("other", "something.other"),
								Str("value", "a"),
							),
							Block("my_block",
								Number("key", 1),
								Expr("other", "something.other"),
								Str("value", "b"),
							),
							Block("my_block",
								Number("key", 2),
								Expr("other", "something.other"),
								Str("value", "c"),
							),
						),
					},
				},
			},
		},
		{
			name:  "tm_dynamic ignored when condition evaluates to false",
			stack: "/stack",
			configs: []hclconfig{
				{
					path:     "/stack",
					filename: "condition.tm",
					add: GenerateHCL(
						Labels("tm_dynamic_test.tf"),
						Content(
							Str("data", "data"),
							TmDynamic(
								Labels("ignored"),
								Expr("for_each", `["a", "b", "c"]`),
								Bool("condition", false),
								Content(
									Expr("value", "b.value"),
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
						condition: true,
						body: Doc(
							Str("data", "data"),
						),
					},
				},
			},
		},
		{
			name:  "inner tm_dynamic blocks ignored when condition of parent evaluates to false",
			stack: "/stack",
			configs: []hclconfig{
				{
					path:     "/stack",
					filename: "condition.tm",
					add: GenerateHCL(
						Labels("tm_dynamic_test.tf"),
						Content(
							Str("data", "data"),
							TmDynamic(
								Labels("ignored"),
								Expr("for_each", `["a", "b", "c"]`),
								Bool("condition", false),
								Content(
									Expr("value", "b.value"),
									TmDynamic(
										Labels("not ignored"),
										Expr("for_each", `["a", "b", "c"]`),
										Bool("condition", true),
										Content(
											Expr("value", "b.value"),
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
						condition: true,
						body: Doc(
							Str("data", "data"),
						),
					},
				},
			},
		},
		{
			name:  "tm_dynamic for_each not evaluated when condition evaluates to false",
			stack: "/stack",
			configs: []hclconfig{
				{
					path:     "/stack",
					filename: "condition.tm",
					add: GenerateHCL(
						Labels("tm_dynamic_test.tf"),
						Content(
							Str("data", "data"),
							TmDynamic(
								Labels("ignored"),
								Expr("for_each", `global.list`),
								Expr("condition", `tm_can(global.list)`),
								Content(
									Expr("value", "b.value"),
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
						condition: true,
						body: Doc(
							Str("data", "data"),
						),
					},
				},
			},
		},
		{
			name:  "fails if condition fails to evaluate",
			stack: "/stack",
			configs: []hclconfig{
				{
					path:     "/stack",
					filename: "condition.tm",
					add: GenerateHCL(
						Labels("tm_dynamic_test.tf"),
						Content(
							Str("data", "data"),
							TmDynamic(
								Labels("ignored"),
								Expr("for_each", `global.list`),
								Expr("condition", `unknown.something`),
								Content(
									Expr("value", "b.value"),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrDynamicConditionEval),
		},
		{
			name:  "fails if condition is not boolean",
			stack: "/stack",
			configs: []hclconfig{
				{
					path:     "/stack",
					filename: "condition.tm",
					add: GenerateHCL(
						Labels("tm_dynamic_test.tf"),
						Content(
							Str("data", "data"),
							TmDynamic(
								Labels("ignored"),
								Expr("for_each", `global.list`),
								Str("condition", `not boolean`),
								Content(
									Expr("value", "b.value"),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrDynamicConditionEval),
		},
		{
			name:  "content with no for_each",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("tm_dynamic_test.tf"),
						Content(
							TmDynamic(
								Labels("my_block"),
								Content(
									Expr("other", "something.other"),
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
						condition: true,
						body: Doc(
							Block("my_block",
								Expr("other", "something.other"),
							),
						),
					},
				},
			},
		},
		{
			name:  "attributes with no for_each defined",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("tm_dynamic_test.tf"),
						Content(
							TmDynamic(
								Labels("attributes"),
								Expr("attributes", `{
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
						condition: true,
						body: Doc(
							Block("attributes",
								Expr("other", "something.other"),
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
					add: GenerateHCL(
						Labels("empty.tf"),
						Content(
							TmDynamic(
								Labels("empty"),
								Expr("for_each", `["a", "b"]`),
								Expr("attributes", `{}`),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "empty.tf",
					hcl: genHCL{
						condition: true,
						body: Doc(
							Block("empty"),
							Block("empty"),
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
					add: GenerateHCL(
						Labels("tm_dynamic_test.tf"),
						Content(
							TmDynamic(
								Labels("attributes"),
								Expr("for_each", `["a", "b", "c"]`),
								Expr("iterator", "iter"),
								Expr("attributes", `{
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
						condition: true,
						body: Doc(
							Block("attributes",
								Str("value", "a"),
								Number("key", 0),
								Expr("other", "something.other"),
							),
							Block("attributes",
								Str("value", "b"),
								Number("key", 1),
								Expr("other", "something.other"),
							),
							Block("attributes",
								Str("value", "c"),
								Number("key", 2),
								Expr("other", "something.other"),
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
					add: Globals(
						Expr("obj", `{
						  a = "global data"
						}`),
					),
				},
				{
					path:     "/stack",
					filename: "gen.tm",
					add: GenerateHCL(
						Labels("test.tf"),
						Content(
							TmDynamic(
								Labels("test"),
								Expr("for_each", `["a"]`),
								Expr("iterator", "iter"),
								Expr("attributes", `tm_merge(global.obj, {
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
						condition: true,
						body: Doc(
							Block("test",
								Str("a", "global data"),
								Number("b", 666),
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
					add: GenerateHCL(
						Labels("tm_dynamic_attributes_order.tf"),
						Content(
							TmDynamic(
								Labels("attributes_order"),
								Expr("for_each", `["test", "test2"]`),
								Expr("iterator", "iter"),
								Expr("attributes", `{
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
						condition: true,
						body: Doc(
							Block("attributes_order",
								Str("b", "test"),
								Number("z", 0),
								Expr("a", "something.other"),
							),
							Block("attributes_order",
								Str("b", "test2"),
								Number("z", 1),
								Expr("a", "something.other"),
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
					add: Globals(
						Str("data", "global string"),
					),
				},
				{
					path:     "/stack",
					filename: "generate.tm",
					add: GenerateHCL(
						Labels("tm_dynamic.tf"),
						Content(
							TmDynamic(
								Labels("references"),
								Expr("for_each", `["test"]`),
								Expr("attributes", `{
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
						condition: true,
						body: Doc(
							Block("references",
								Str("global", "global string"),
								Str("meta", "/stack"),
								Str("interp", "GLOBAL STRING INTERP"),
								Expr("partial", "local.data"),
								Str("iter", "test"),
								Expr("partialfunc", `upper("global string")`),
								Expr("ternary", `local.cond ? local.val : "global string"`),
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
					add: Globals(
						Str("key", "globalkey"),
					),
				},
				{
					path:     "/stack",
					filename: "generate.tm",
					add: GenerateHCL(
						Labels("tm_dynamic.tf"),
						Content(
							TmDynamic(
								Labels("references"),
								Expr("for_each", `["test"]`),
								Expr("attributes", `{
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
						condition: true,
						body: Doc(
							Block("references",
								Bool("globalkey", true),
								Bool("stack", true),
								Bool("GLOBALKEY", true),
								Bool("test", true),
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
					add: GenerateHCL(
						Labels("tm_dynamic_test.tf"),
						Content(
							TmDynamic(
								Labels("parent"),
								Expr("for_each", `["a", "b"]`),
								Content(
									Expr("value", "parent.value"),
									Expr("key", "parent.key"),
									Expr("other", "something.other"),
									TmDynamic(
										Labels("child"),
										Expr("for_each", `[0, 1]`),
										Expr("attributes", `{
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
						condition: true,
						body: Doc(
							Block("parent",
								Number("key", 0),
								Expr("other", "something.other"),
								Str("value", "a"),
								Block("child",
									Str("value", "0-a-0"),
								),
								Block("child",
									Str("value", "0-a-1"),
								),
							),
							Block("parent",
								Number("key", 1),
								Expr("other", "something.other"),
								Str("value", "b"),
								Block("child",
									Str("value", "1-b-0"),
								),
								Block("child",
									Str("value", "1-b-1"),
								),
							),
						),
					},
				},
			},
		},
		{
			name:  "no for_each with iterator definition",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("tm_dynamic_test.tf"),
						Content(
							TmDynamic(
								Labels("my_block"),
								Expr("iterator", "iter"),
								Content(
									Expr("value", "b.value"),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrInvalidDynamicIterator),
		},
		{
			name:  "tm_dynamic inside tm_dynamic using content",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("tm_dynamic_test.tf"),
						Content(
							TmDynamic(
								Labels("my_block"),
								Expr("for_each", `["a", "b", "c"]`),
								Expr("iterator", "b"),
								Content(
									Expr("value", "b.value"),
									Expr("key", "b.key"),
									Expr("other", "something.other"),
									TmDynamic(
										Labels("child"),
										Expr("for_each", `[0, 1, 2]`),
										Expr("iterator", "i"),
										Content(
											Str("value", "${b.key}-${b.value}-${i.value}"),
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
						condition: true,
						body: Doc(
							Block("my_block",
								Number("key", 0),
								Expr("other", "something.other"),
								Str("value", "a"),
								Block("child",
									Str("value", "0-a-0"),
								),
								Block("child",
									Str("value", "0-a-1"),
								),
								Block("child",
									Str("value", "0-a-2"),
								),
							),
							Block("my_block",
								Number("key", 1),
								Expr("other", "something.other"),
								Str("value", "b"),
								Block("child",
									Str("value", "1-b-0"),
								),
								Block("child",
									Str("value", "1-b-1"),
								),
								Block("child",
									Str("value", "1-b-2"),
								),
							),
							Block("my_block",
								Number("key", 2),
								Expr("other", "something.other"),
								Str("value", "c"),
								Block("child",
									Str("value", "2-c-0"),
								),
								Block("child",
									Str("value", "2-c-1"),
								),
								Block("child",
									Str("value", "2-c-2"),
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
					add: Doc(
						Globals(
							Str("msg", "hello"),
							Expr("values", `["a", "b", "c"]`),
						),
						GenerateHCL(
							Labels("tm_dynamic_test.tf"),
							Content(
								TmDynamic(
									Labels("my_block"),
									Expr("for_each", `global.values`),
									Content(
										Expr("msg", `global.msg`),
										Expr("val", `global.values[my_block.key]`),
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
						condition: true,
						body: Doc(
							Block("my_block",
								Str("msg", "hello"),
								Str("val", "a"),
							),
							Block("my_block",
								Str("msg", "hello"),
								Str("val", "b"),
							),
							Block("my_block",
								Str("msg", "hello"),
								Str("val", "c"),
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
					add: GenerateHCL(
						Labels("tm_dynamic_test.tf"),
						Content(
							TmDynamic(
								Labels("my_block"),
								Expr("for_each", `["a", "b", "c"]`),
								Expr("iterator", "[]"),
								Content(
									Expr("value", "b.value"),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrInvalidDynamicIterator),
		},
		{
			name:  "no content block and no attributes fails",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("tm_dynamic_test.tf"),
						Content(
							TmDynamic(
								Labels("my_block"),
								Expr("for_each", `["a", "b", "c"]`),
							),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrParsing),
		},
		{
			name:  "attributes is null fails",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("tm_dynamic_test.tf"),
						Content(
							TmDynamic(
								Labels("my_block"),
								Expr("for_each", `["a"]`),
								Expr("attributes", "null"),
							),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrParsing),
		},
		{
			name:  "attributes is not object fails",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("tm_dynamic_test.tf"),
						Content(
							TmDynamic(
								Labels("my_block"),
								Expr("for_each", `["a"]`),
								Expr("attributes", "[]"),
							),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrParsing),
		},
		{
			name:  "attributes key is undefined fails",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("fail.tf"),
						Content(
							TmDynamic(
								Labels("my_block"),
								Expr("for_each", `["a"]`),
								Expr("attributes", `{ (local.a) : 666 }`),
							),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrDynamicAttrsEval),
		},
		{
			name:  "attributes key is not a string fails",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						Globals(
							Number("a", 666),
						),
						GenerateHCL(
							Labels("fail.tf"),
							Content(
								TmDynamic(
									Labels("my_block"),
									Expr("for_each", `["a"]`),
									Expr("attributes", `{ (global.a) : 666 }`),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrParsing),
		},
		{
			name:  "attributes key has space fails",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						GenerateHCL(
							Labels("fail.tf"),
							Content(
								TmDynamic(
									Labels("my_block"),
									Expr("for_each", `["a"]`),
									Expr("attributes", `{ "spaces not allowed" : 666 }`),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrParsing),
		},
		{
			name:  "attributes key is empty string fails",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						GenerateHCL(
							Labels("fail.tf"),
							Content(
								TmDynamic(
									Labels("my_block"),
									Expr("for_each", `["a"]`),
									Expr("attributes", `{ "" : 666 }`),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrParsing),
		},
		{
			name:  "attributes key starts with '-' fails",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						GenerateHCL(
							Labels("fail.tf"),
							Content(
								TmDynamic(
									Labels("my_block"),
									Expr("for_each", `["a"]`),
									Expr("attributes", `{ "-name" : 666 }`),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrParsing),
		},
		{
			name:  "attributes and unknown attribute fails",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("tm_dynamic_test.tf"),
						Content(
							TmDynamic(
								Labels("my_block"),
								Expr("for_each", `["a", "b", "c"]`),
								Expr("attributes", `{ b : 666 }`),
								Str("something", "val"),
							),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrParsing),
		},
		{
			name:  "tm_dynamic with more than one label fails",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("tm_dynamic_test.tf"),
						Content(
							TmDynamic(
								Labels("my_block", "nope"),
								Expr("for_each", `["a"]`),
								Content(
									Str("a", "val"),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrParsing),
		},
		{
			name:  "tm_dynamic with multiple content blocks fail",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("tm_dynamic_test.tf"),
						Content(
							TmDynamic(
								Labels("my_block"),
								Expr("for_each", `["a"]`),
								Content(
									Str("a", "val"),
								),
								Content(
									Str("a", "val"),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrParsing),
		},
		{
			name:  "tm_dynamic with unknown block fail",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("tm_dynamic_test.tf"),
						Content(
							TmDynamic(
								Labels("my_block"),
								Expr("for_each", `["a"]`),
								Content(
									Str("a", "val"),
								),
								Block("unsupported",
									Str("a", "val"),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrParsing),
		},
		{
			name:  "tm_dynamic with undefined for_each fail",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("tm_dynamic_test.tf"),
						Content(
							TmDynamic(
								Labels("my_block"),
								Expr("for_each", `[local.a]`),
								Content(
									Str("a", "val"),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrParsing),
		},
		{
			name:  "tm_dynamic with labels with undefined references fails",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("tm_dynamic_test.tf"),
						Content(
							TmDynamic(
								Labels("my_block"),
								Expr("for_each", `["a"]`),
								Expr("labels", "[local.a]"),
								Content(
									Str("a", "val"),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrInvalidDynamicLabels),
		},
		{
			name:  "tm_dynamic with labels that is not a list fails",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("tm_dynamic_test.tf"),
						Content(
							TmDynamic(
								Labels("my_block"),
								Expr("for_each", `["a"]`),
								Expr("labels", "{}"),
								Content(
									Str("a", "val"),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrInvalidDynamicLabels),
		},
		{
			name:  "tm_dynamic with iterator with traversal fails",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("tm_dynamic_test.tf"),
						Content(
							TmDynamic(
								Labels("my_block"),
								Expr("for_each", `["a"]`),
								Expr("iterator", "name.traverse"),
								Content(
									Str("a", "val"),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrInvalidDynamicIterator),
		},
		{
			name:  "tm_dynamic with undefined global on attributes fails",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("tm_dynamic_test.tf"),
						Content(
							TmDynamic(
								Labels("my_block"),
								Expr("for_each", `["a"]`),
								Expr("attributes", `{ a = global.undefined }`),
							),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrDynamicAttrsEval),
		},
		{
			name:  "tm_dynamic with undefined global on content fails",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("tm_dynamic_test.tf"),
						Content(
							TmDynamic(
								Labels("my_block"),
								Expr("for_each", `["a"]`),
								Content(
									Expr("a", `{ a = global.undefined }`),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(eval.ErrPartial),
		},
		{
			name:  "tm_dynamic with non-iterable for_each fail",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("tm_dynamic_test.tf"),
						Content(
							TmDynamic(
								Labels("my_block"),
								Number("for_each", 666),
								Content(
									Str("a", "val"),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrParsing),
		},
		{
			name:  "tm_dynamic with no label fails",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("tm_dynamic_test.tf"),
						Content(
							TmDynamic(
								Expr("for_each", `["a"]`),
								Content(
									Str("a", "val"),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrParsing),
		},
		{
			name:  "content and unknown attribute fails",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("tm_dynamic_test.tf"),
						Content(
							TmDynamic(
								Labels("my_block"),
								Expr("for_each", `["a", "b", "c"]`),
								Str("something", "val"),
								Content(
									Str("a", "val"),
								),
							),
						),
					),
				},
			},
			wantErr: errors.E(genhcl.ErrParsing),
		},
		{
			name:  "content block and attributes fails",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("tm_dynamic_test.tf"),
						Content(
							TmDynamic(
								Labels("my_block"),
								Expr("for_each", `["a", "b", "c"]`),
								Expr("attributes", `{ b : 666 }`),
								Content(
									Str("a", "val"),
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
