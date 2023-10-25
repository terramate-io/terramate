// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package genhcl_test

import (
	"testing"

	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/lets"
	maptest "github.com/terramate-io/terramate/mapexpr/test"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
)

func TestGenHCLLetsMapSchemaErrors(t *testing.T) {
	t.Parallel()
	for _, maptc := range maptest.SchemaErrorTestcases() {
		tc := testcase{
			name:  "genhcl with lets and " + maptc.Name,
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateFile(
						Labels("test.tf"),
						Lets(
							Str("name", "value"),
							maptc.Block,
						),
						Content(
							Expr("name", "let.name"),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		}
		tc.run(t)
	}
}

func TestGenHCLLetsMap(t *testing.T) {
	t.Parallel()

	for _, tc := range []testcase{
		{
			name:  "lets.map label conflicts with lets name",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("test.tf"),
						Lets(
							Str("name", "value"),
							Map(
								Labels("name"),
								Expr("for_each", "[]"),
								Str("key", "a"),
								Str("value", "a"),
							),
						),
						Content(
							Expr("name", "let.name"),
						),
					),
				},
			},
			wantErr: errors.E(lets.ErrRedefined),
		},
		{
			name:  "lets with simple map block",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("test.tf"),
						Lets(
							Map(
								Labels("var"),
								Expr("for_each", `["a", "b", "c"]`),
								Expr("key", "element.new"),
								Expr("value", "element.new"),
							),
						),
						Content(
							Str("content", "${let.var.a}-${let.var.b}-${let.var.c}"),
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
							Str("content", "a-b-c"),
						),
					},
				},
			},
		},
		{
			name:  "lets with map block using iterator",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("test.tf"),
						Lets(
							Map(
								Labels("var"),
								Expr("iterator", "el"),
								Expr("for_each", `["a", "b", "c"]`),
								Expr("key", "el.new"),
								Expr("value", "el.new"),
							),
						),
						Content(
							Str("content", "${let.var.a}-${let.var.b}-${let.var.c}"),
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
							Str("content", "a-b-c"),
						),
					},
				},
			},
		},
		{
			name:  "lets with map block with incorrect key",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("test.tf"),
						Lets(
							Map(
								Labels("var"),
								Expr("for_each", `["a", "b", "c"]`),
								Expr("key", "something"), // keyword is not valid
								Str("value", "else"),
							),
						),
						Content(
							Str("content", "${let.var.a}-${let.var.b}-${let.var.c}"),
						),
					),
				},
			},
			wantErr: errors.E(lets.ErrEval),
		},
		{
			name:  "lets with map block with incorrect value",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("test.tf"),
						Lets(
							Map(
								Labels("var"),
								Expr("for_each", `["a", "b", "c"]`),
								Expr("value", "something"), // keyword is not valid
								Str("key", "else"),
							),
						),
						Content(
							Str("content", "${let.var.a}-${let.var.b}-${let.var.c}"),
						),
					),
				},
			},
			wantErr: errors.E(lets.ErrEval),
		},
		{
			name:  "lets with map block without using element",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("test.tf"),
						Lets(
							Map(
								Labels("var"),
								Expr("for_each", `["a", "b", "c"]`),
								Str("key", "something"),
								Str("value", "else"),
							),
						),
						Content(
							Expr("content", "let.var.something"),
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
							Str("content", "else"),
						),
					},
				},
			},
		},
		{
			name:  "lets with multiple map blocks",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("test.tf"),
						Lets(
							Map(
								Labels("var1"),
								Expr("for_each", `["a", "b", "c"]`),
								Expr("key", "element.new"),
								Expr("value", "element.new"),
							),
							Map(
								Labels("var2"),
								Expr("for_each", `["d", "e", "f"]`),
								Expr("key", "element.new"),
								Expr("value", "element.new"),
							),
						),
						Content(
							Str("var1", "${let.var1.a}-${let.var1.b}-${let.var1.c}"),
							Str("var2", "${let.var2.d}-${let.var2.e}-${let.var2.f}"),
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
							Str("var1", "a-b-c"),
							Str("var2", "d-e-f"),
						),
					},
				},
			},
		},
		{
			name:  "lets map unknowns are postponed in the evaluator",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("test.tf"),
						Lets(
							Str("val3", "val3"),
							Map(
								Labels("val"),
								Expr("iterator", "el"),
								Expr("for_each", `[let.val1, let.val2, let.val3]`),
								Expr("key", "el.new"),
								Expr("value", "el.new"),
							),
							Str("val2", "val2"),
							Str("val1", "val1"),
						),
						Content(
							Str("val", "${let.val.val1}-${let.val.val2}-${let.val.val3}"),
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
							Str("val", "val1-val2-val3"),
						),
					},
				},
			},
		},
	} {
		tc.run(t)
	}
}
