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
			name:  "tm_dynamic inside tm_dynamic",
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
			name:  "tm_dynamic with no for_each",
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
	}

	for _, tcase := range tcases {
		tcase.run(t)
	}
}
