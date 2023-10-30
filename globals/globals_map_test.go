// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package globals_test

import (
	"testing"

	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/globals"
	"github.com/terramate-io/terramate/hcl"
	maptest "github.com/terramate-io/terramate/mapexpr/test"
	"github.com/terramate-io/terramate/test/hclwrite"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
)

func TestGlobalsWithMapSchemaErrors(t *testing.T) {
	t.Parallel()
	for _, mapcase := range maptest.SchemaErrorTestcases() {
		tc := testcase{
			name: "globals with " + mapcase.Name,
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					filename: "global.tm",
					path:     "/stack",
					add: Globals(
						mapcase.Block,
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		}
		testGlobals(t, tc)
	}
}

func TestGlobalsMap(t *testing.T) {
	t.Parallel()

	for _, tc := range []testcase{
		{
			name:   "globals.map label conflicts with global name",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Str("name", "test"),
						Map(
							Labels("name"),
							Expr("for_each", "[]"),
							Str("key", "a"),
							Str("value", "a"),
						),
					),
				},
			},
			wantErr: errors.E(globals.ErrRedefined),
		},
		{
			name:   "invalid globals.map key",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Map(
							Labels("var"),
							Expr("for_each", `["a", "b", "c"]`),
							Expr("key", "something"), // keyword, not a string
							Str("value", "else"),
						),
					),
				},
			},
			wantErr: errors.E(globals.ErrEval),
		},
		{
			name:   "invalid globals.map value",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Map(
							Labels("var"),
							Expr("for_each", `["a", "b", "c"]`),
							Str("key", "something"),
							Expr("value", "else"), // keyword, not an expression
						),
					),
				},
			},
			wantErr: errors.E(globals.ErrEval),
		},
		{
			name:   "simple globals.map without using element",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Map(
							Labels("var"),
							Expr("for_each", `["a", "b", "c"]`),
							Str("key", "something"),
							Str("value", "else"),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "var", `{
						something = "else"
					}`),
				),
			},
		},
		{
			name:   "conflicting globals.map name with other globals",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Str("var", "test"),
						Map(
							Labels("var"),
							Expr("for_each", `["a", "b", "c"]`),
							Expr("key", "element.new"),
							Expr("value", "element.new"),
						),
					),
				},
			},
			wantErr: errors.E(globals.ErrRedefined),
		},
		{
			name:   "simple globals.map ",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Map(
							Labels("var"),
							Expr("for_each", `["a", "b", "c"]`),
							Expr("key", "element.new"),
							Expr("value", "element.new"),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "var", `{
						a = "a"
						b = "b"
						c = "c"
					}`),
				),
			},
		},
		{
			name:   "multiple globals.map blocks",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Map(
							Labels("var"),
							Expr("for_each", `["a", "b", "c"]`),
							Expr("key", "element.new"),
							Expr("value", "element.new"),
						),
						Map(
							Labels("var2"),
							Expr("for_each", `["a", "b", "c"]`),
							Expr("key", "element.new"),
							Expr("value", "element.new"),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "var", `{
						a = "a"
						b = "b"
						c = "c"
					}`),
					EvalExpr(t, "var2", `{
						a = "a"
						b = "b"
						c = "c"
					}`),
				),
			},
		},
		{
			name:   "simple globals.map with different iterator",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Map(
							Labels("var"),
							Expr("iterator", "el"),
							Expr("for_each", `["a", "b", "c"]`),
							Expr("key", "el.new"),
							Expr("value", "el.new"),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "var", `{
						a = "a"
						b = "b"
						c = "c"
					}`),
				),
			},
		},
		{
			name:   "globals.map unknowns are postponed in the evaluator",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Map(
							Labels("var"),
							Expr("iterator", "el"),
							Expr("for_each", `[global.val1, global.val2, global.val3]`),
							Expr("key", "el.new"),
							Expr("value", "el.new"),
						),
						Str("val2", "val2"),
					),
				},
				{
					path: "/",
					add: Globals(
						Str("val1", "val1"),
						Str("val3", "val3"),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					Str("val1", "val1"),
					Str("val2", "val2"),
					Str("val3", "val3"),
					EvalExpr(t, "var", `{
						val1 = "val1"
						val2 = "val2"
						val3 = "val3"
					}`),
				),
			},
		},
		{
			name:   "globals.map unknowns are postponed in the evaluator even when parent depends on child",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Map(
							Labels("var"),
							Expr("iterator", "el"),
							Expr("for_each", `[global.val1, global.val2, global.val3]`),
							Expr("key", "el.new"),
							Expr("value", "el.new"),
						),
						Str("val2", "val2"),
					),
				},
				{
					path: "/stack",
					add: Globals(
						Str("val1", "val1"),
						Str("val3", "val3"),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					Str("val1", "val1"),
					Str("val2", "val2"),
					Str("val3", "val3"),
					EvalExpr(t, "var", `{
						val1 = "val1"
						val2 = "val2"
						val3 = "val3"
					}`),
				),
			},
		},
		{
			name:   "element.old is undefined on first iteration of a given key",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Map(
							Labels("people_count"),
							Expr("for_each", `["marius", "tiago", "soeren", "tiago"]`),
							Expr("key", "element.new"),
							Expr("value", `tm_try(element.old, 0) + 1`),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "people_count", `{
						marius = 1
  						tiago  = 2
  						soeren = 1
					}`),
				),
			},
		},
		{
			name:   "using element.old in value attr to count people",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Map(
							Labels("people_count"),
							Expr("for_each", `["marius", "tiago", "soeren", "tiago"]`),
							Expr("key", "element.new"),
							Expr("value", `{
								count = tm_try(element.old.count, 0) + 1	
							}`),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "people_count", `{
						marius = {count = 1}
  						tiago  = {count = 2}
  						soeren = {count = 1}
					}`),
				),
			},
		},
		{
			name:   "using element.old in value block to count people",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Map(
							Labels("people_count"),
							Expr("for_each", `["marius", "tiago", "soeren", "tiago"]`),
							Expr("key", "element.new"),
							Value(
								Expr("count", `tm_try(element.old.count, 0) + 1`),
							),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "people_count", `{
						marius = {count = 1}
  						tiago  = {count = 2}
  						soeren = {count = 1}
					}`),
				),
			},
		},
		{
			name:   "globals.map is recursive",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Expr("lst", `["a", "b", "c"]`),
						Map(
							Labels("var"),
							Expr("for_each", `global.lst`),
							Expr("key", "element.new"),

							Value(
								Str("some", "value"),
								Map(
									Labels("var"),
									Expr("for_each", "global.lst"),
									Expr("key", "element.new"),
									Expr("value", "element.new"),
								),
							),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "lst", `["a", "b", "c"]`),
					EvalExpr(t, "var", `{
						a = {
							some = "value"
							var = {
								a = "a"
								b = "b"
								c = "c"
							}
						}
						b = {
							some = "value"
							var = {
								a = "a"
								b = "b"
								c = "c"
							}
						}
						c = {
							some = "value"
							var = {
								a = "a"
								b = "b"
								c = "c"
							}
						}
					}`),
				),
			},
		},
		{
			name:   "recursive globals.map with multiple map blocks",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Expr("lst", `["a", "b", "c"]`),
						Map(
							Labels("var"),
							Expr("for_each", `global.lst`),
							Expr("key", "element.new"),

							Value(
								Str("some", "value"),
								Map(
									Labels("var"),
									Expr("for_each", "global.lst"),
									Expr("key", "element.new"),
									Expr("value", "element.new"),
								),
								Map(
									Labels("var2"),
									Expr("for_each", "global.lst"),
									Expr("key", "element.new"),
									Expr("value", "element.new"),
								),
							),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "lst", `["a", "b", "c"]`),
					EvalExpr(t, "var", `{
						a = {
							some = "value"
							var = {
								a = "a"
								b = "b"
								c = "c"
							}
							var2 = {
								a = "a"
								b = "b"
								c = "c"
							}
						}
						b = {
							some = "value"
							var = {
								a = "a"
								b = "b"
								c = "c"
							}
							var2 = {
								a = "a"
								b = "b"
								c = "c"
							}
						}
						c = {
							some = "value"
							var = {
								a = "a"
								b = "b"
								c = "c"
							}
							var2 = {
								a = "a"
								b = "b"
								c = "c"
							}
						}
					}`),
				),
			},
		},
		{
			name:   "recursive globals.map reusing element iterator",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Expr("lst", `[
							{
								val: "a", 
								lst: [1, 2, 3]
							},
							{
								val: "b",
								lst: [4, 5, 6]
							},
							{
								val: "c",
								lst: [7, 8, 9]
							}
						]`),
						Map(
							Labels("var"),
							Expr("for_each", `global.lst`),
							Expr("key", "element.new.val"),

							Value(
								Str("some", "value"),
								Map(
									Labels("var"),
									Expr("for_each", "element.new.lst"),
									Expr("key", "tm_tostring(element.new)"),
									Expr("value", "element.new"),
								),
								Map(
									Labels("var2"),
									Expr("for_each", "element.new.lst"),
									Expr("key", "tm_tostring(element.new)"),
									Expr("value", "element.new"),
								),
							),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "lst", `[
						{
							val: "a", 
							lst: [1, 2, 3]
						},
						{
							val: "b",
							lst: [4, 5, 6]
						},
						{
							val: "c",
							lst: [7, 8, 9]
						}
					]`),
					EvalExpr(t, "var", `{
						a = {
							some = "value"
							var = {
								"1" = 1
								"2" = 2
								"3" = 3
							}
							var2 = {
								"1" = 1
								"2" = 2
								"3" = 3
							}
						}
						b = {
							some = "value"
							var = {
								"4" = 4
								"5" = 5
								"6" = 6
							}
							var2 = {
								"4" = 4
								"5" = 5
								"6" = 6
							}
						}
						c = {
							some = "value"
							var = {
								"7" = 7
								"8" = 8
								"9" = 9
							}
							var2 = {
								"7" = 7
								"8" = 8
								"9" = 9
							}
						}
					}`),
				),
			},
		},
		{
			name:   "recursive globals.map reusing with different nested iterator",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Expr("lst", `[
							{
								val: "a", 
								lst: [1, 2, 3]
							},
							{
								val: "b",
								lst: [4, 5, 6]
							},
							{
								val: "c",
								lst: [7, 8, 9]
							}
						]`),
						Map(
							Labels("var"),
							Expr("for_each", `global.lst`),
							Expr("key", "element.new.val"),

							Value(
								Str("some", "value"),
								Map(
									Labels("var"),
									Expr("for_each", "element.new.lst"),
									Expr("key", "tm_tostring(el.new)"),
									Expr("value", "el.new"),
									Expr("iterator", "el"),
								),
								Map(
									Labels("var2"),
									Expr("for_each", "element.new.lst"),
									Expr("key", "tm_tostring(el2.new)"),
									Expr("value", "el2.new"),
									Expr("iterator", "el2"),
								),
							),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "lst", `[
						{
							val: "a", 
							lst: [1, 2, 3]
						},
						{
							val: "b",
							lst: [4, 5, 6]
						},
						{
							val: "c",
							lst: [7, 8, 9]
						}
					]`),
					EvalExpr(t, "var", `{
						a = {
							some = "value"
							var = {
								"1" = 1
								"2" = 2
								"3" = 3
							}
							var2 = {
								"1" = 1
								"2" = 2
								"3" = 3
							}
						}
						b = {
							some = "value"
							var = {
								"4" = 4
								"5" = 5
								"6" = 6
							}
							var2 = {
								"4" = 4
								"5" = 5
								"6" = 6
							}
						}
						c = {
							some = "value"
							var = {
								"7" = 7
								"8" = 8
								"9" = 9
							}
							var2 = {
								"7" = 7
								"8" = 8
								"9" = 9
							}
						}
					}`),
				),
			},
		},
	} {
		testGlobals(t, tc)
	}
}
