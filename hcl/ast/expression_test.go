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
			expr: `test(1, 2, 2.5, "test", 1, another_funcall(1, 2, 3))`,
		},
		{
			name: "funcall with complex args",
			expr: `test({
				a = 1
				fn = funcall(1, 2)
			}, [{}, {
				a = 2
				}, 3, [2, 3, 4]], 2.5, "test", 1, {
				name = "terramate"
			})`,
		},
		{
			name: "namespace",
			expr: `abc`,
		},
		{
			name: "traversal",
			expr: `abc.xyz`,
		},
		{
			name: "namespace with number indexing",
			expr: `abc[0]`,
		},
		{
			name: "namespace with namespace indexing",
			expr: `abc[xyz]`,
		},
		{
			name: "namespace with namespace with namespace indexing",
			expr: `abc[xyz[xpto]]`,
		},
		{
			name: "namespace with indexing traversal",
			expr: `abc[xyz.xpto]`,
		},
		{
			name: "namespace with indexing traversal with indexing",
			expr: `abc[xyz.xpto[0]]`,
		},
		{
			name: "simple splat",
			expr: `abc[*]`,
		},
		{
			name: "splat with attr selection",
			expr: `abc[*].id`,
		},
		{
			name: "splat with traversal selection",
			expr: `abc[*].a.b.c.d`,
		},
		{
			name: "arithmetic binary operation (+)",
			expr: `1+1`,
		},
		{
			name: "arithmetic binary operation (-)",
			expr: `1-1`,
		},
		{
			name: "arithmetic binary operation (/)",
			expr: `1/1`,
		},
		{
			name: "arithmetic binary operation (*)",
			expr: `1*1`,
		},
		{
			name: "arithmetic binary operation (%)",
			expr: `1%1`,
		},
		{
			name: "arithmetic n-ary operation (+)",
			expr: `1+1+2+3+5+8+13+21`,
		},
		{
			name: "arithmetic unary operation (-)",
			expr: `-1`,
		},
		{
			name: "logical binary operation (==)",
			expr: `1==1`,
		},
		{
			name: "logical binary operation (!=)",
			expr: `1!=1`,
		},
		{
			name: "logical binary operation (<)",
			expr: `1<1`,
		},
		{
			name: "logical binary operation (>)",
			expr: `1>1`,
		},
		{
			name: "logical binary operation (<=)",
			expr: `1<=1`,
		},
		{
			name: "logical binary operation (>=)",
			expr: `1>=1`,
		},
		{
			name: "logical operation (!)",
			expr: `!true`,
		},
		{
			name: "logical operation (&&)",
			expr: `false && false`,
		},
		{
			name: "logical operation (||)",
			expr: `false || true`,
		},
		{
			name: "logical operation - n-ary (||)",
			expr: `a || b || c || d`,
		},
		{
			name: "logical operation - n-ary (&&)",
			expr: `a && b && c && d`,
		},
		{
			name: "parenthesis operations - unary - digit",
			expr: `(1)`,
		},
		{
			name: "parenthesis operations - unary - ident",
			expr: `(a)`,
		},
		{
			name: "parenthesis operations - unary - string",
			expr: `("test")`,
		},
		{
			name: "n-parenthesis operations - unary - ident",
			expr: `((a))`,
		},
		{
			name: "parenthesis operations - binary",
			expr: `(a+1)`,
		},
		{
			name: "basic conditional",
			expr: `a ? b : c`,
		},
		{
			name: "conditional with nesting",
			expr: `a ? x ? y : z : c`,
		},
		{
			name: "parenthesis with conditional with nesting",
			expr: `a ? (x ? y : z) : c`,
		},
		{
			name: "for-expr - list",
			expr: `[for a in b : c]`,
		},
		{
			name: "for-expr - list with exprs",
			expr: `[for a in func() : func()]`,
		},
		{
			name: "for-expr - list with cond",
			expr: `[for a in func() : func() if cond()]`,
		},
		{
			name: "for-expr - object",
			expr: `{for k,v in c : k => v}`,
		},
		{
			name: "for-expr - object with exprs and cond",
			expr: `{for k,v in expr() : expr()+test() => expr()+test()+1 if expr()}`,
		},
		{
			name: "all-in-one",
			expr: `[{
				a = [{
						b = c.d+2+test()
						c = a && b || c && !d || a ? b : c
						d = a+b-c*2/3+!2+test(1, 2, 3)
						c = {for k,v in a.b.c : a() => b() if c}
						d = [for v in a.b.c : a() if b ]
					}, ["test", 1, {}],	func({}, [], "", 1, 2)]
				b = x.y[*].z
				c = a[0]
				d = a[b.c[d.e[*].a]]
			}]`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			expr, diags := hclsyntax.ParseExpression([]byte(tc.expr), "test.hcl", hcl.InitialPos)
			assert.IsTrue(t, !diags.HasErrors(), diags.Error())
			got := ast.TokensForExpression(expr)
			fmtWant := string(hclwrite.Format([]byte(tc.expr)))
			fmtGot := string(hclwrite.Format(got.Bytes()))
			assert.EqualStrings(t, fmtWant, fmtGot)
		})
	}
}
