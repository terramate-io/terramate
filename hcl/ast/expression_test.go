package ast_test

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/hcl/ast"
)

func TestAstExpressionToTokens(t *testing.T) {
	type testcase struct {
		name string
		expr string
	}

	for _, tc := range []testcase{
		{
			name: "real numbers",
			expr: "2356.25",
		},
		{
			name: "decimal numbers",
			expr: "2356",
		},
		{
			name: "empty plain strings",
			expr: `""`,
		},
		{
			name: "plain strings",
			expr: `"terramate"`,
		},
		{
			name: "empty list",
			expr: `[]`,
		},
		{
			name: "list with literals",
			expr: `[1, 2, 3, 4, 5]`,
		},
		{
			name: "list with strings",
			expr: `["a", "b", "cc", "ddd", "eeee"]`,
		},
		{
			name: "list with strings and numbers",
			expr: `["a", 1, "b", 2.5, "cc", 3.5, "ddd", 4.5, "eeee"]`,
		},
		{
			name: "empty object",
			expr: `{}`,
		},
		{
			name: "object with level - literals",
			expr: `{
				a.b.c = 1
				b = "test"
				c = 2.5
				d = []
				e = [1, 2, 3]
				f = {}
			}`,
		},
		{
			name: "object with nested values",
			expr: `{
				a = {
					a = {
						a = {}
						b = 1
					}
					b = 1
				}
				b = 1
			}`,
		},
		{
			name: "funcall no args",
			expr: `test()`,
		},
		{
			name: "funcall with literal args",
			expr: `test(1, 2, 2.5, "test", 1)`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			expr, diags := hclsyntax.ParseExpression([]byte(tc.expr), "test.hcl", hcl.InitialPos)
			assert.IsTrue(t, !diags.HasErrors())
			got := ast.TokensForExpression(expr)
			fmtWant := string(hclwrite.Format([]byte(tc.expr)))
			fmtGot := string(hclwrite.Format(got.Bytes()))
			assert.EqualStrings(t, fmtWant, fmtGot)
		})
	}
}
