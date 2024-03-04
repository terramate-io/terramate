// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package ast_test

import (
	"testing"

	"github.com/go-test/deep"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/hcl/ast"
)

func TestAstExpressionToTokens(t *testing.T) {
	t.Parallel()
	type testcase struct {
		name string
		expr string
		want string // if not set, use the input expr
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
			name: "plain strings with escaped strings still plain",
			expr: `"a\tb\\tc\\nd"`,
		},
		{
			name: "string interpolation",
			expr: `"some ${interpolation} here"`,
		},
		{
			name: "string interpolation",
			expr: `"some ${funcall()} here"`,
		},
		{
			name: "interp with number",
			expr: `"0${0}"`,
			want: `"00"`,
		},
		{
			name: "empty heredocs",
			expr: `<<-EOT
EOT
`,
			want: `""`,
		},
		{
			name: "oneline heredocs",
			expr: `<<-EOT

EOT
`,
		},
		{
			name: "heredocs with newlines",
			expr: `<<-EOT
Line 1
Line 2. I want the following \ and n to be in the file: \n
Line 3
EOT
`,
		},
		{
			name: "strings not ending with nl returns as plain string",
			expr: `"0\n1"`,
			want: `"0\n1"`,
		},
		{
			name: "strings ending with nl returns heredocs",
			expr: `"a b c\n"`,
			want: `<<-EOT
a b c
EOT
`,
		},
		{
			name: "heredocs have unespaced quotes",
			expr: `"name=\"val\"\n"`,
			want: `<<-EOT
name="val"
EOT
`,
		},
		{
			name: "heredocs have unescaped backslashes",
			expr: `"\\\n"`,
			want: `<<-EOT
\
EOT
`,
		},
		{
			name: "quoted have unescaped backslashes",
			expr: `"A\\\nB"`,
			want: `"A\\\nB"`,
		},
		{
			name: "escaped newlines",
			expr: `"\\n\\n"`,
		},
		{
			name: "escaped \\ followed by newline at the end",
			expr: `"\\\n"`,
			want: `<<-EOT
\
EOT
`,
		},
		{
			name: "escaped \\ followed by newline in the middle",
			expr: `"\\\n\\\n"`,
			want: `<<-EOT
\
\
EOT
`,
		},
		{
			name: "escaped newlines",
			expr: `"\\n\\n"`,
		},
		{
			name: "heredocs with mixed escaped symbols",
			expr: `"\n\\\n\\\n"`,
			want: `<<-EOT

\
\
EOT
`,
		},
		{
			name: "carriege returns generates plain strings",
			expr: `"\r\n"`,
		},
		{
			name: "escaped newline at the end",
			expr: `"a\\n"`,
		},
		{
			name: "carriege returns generates plain strings 2",
			expr: `"ABC\rDEF"`,
		},
		{
			name: "carriege returns generates plain strings 3",
			expr: `"\r"`,
		},
		{
			name: "non-printable unicode sequences are kept escaped - NULL",
			expr: `"test\u0000\n"`,
		},
		{
			name: "non-printable unicode sequences are kept escaped - SHIFT IN",
			expr: `"test\u000f\n"`,
		},
		{
			name: "non-printable unicode sequences are kept escaped - UNIT SEPARATOR (US) DOWN ARROW",
			expr: `"test\u001f\n"`,
		},
		{
			name: "printable unicodes are rendered",
			expr: `"test\u1000\n"`,
			want: `<<-EOT
testက
EOT
`,
		},
		{
			name: "printable unicodes are rendered when in quoted string",
			expr: `"test\u1000"`,
			want: `"testက"`,
		},
		{
			name: "rendered printable unicodes are also rendered when in quoted string",
			expr: `"testက"`,
			want: `"testက"`,
		},
		{
			name: "single nl returns heredocs",
			expr: `"\n"`,
			want: `<<-EOT

EOT
`,
		},
		{
			name: "strings with multiline interpolation",
			expr: `"test${
				1
			}"`,
			want: `"test1"`,
		},
		{
			name: "strings with multiline interpolation",
			expr: `"test${{
				a = 1
			}}"`,
		},
		{
			name: "strings with multiline HIL interpolation - reformatted",
			expr: `"test${
				global.a == "cond" ? "br1" : "br2"
			}"`,
			want: `"test${global.a == "cond" ? "br1" : "br2"}"`,
		},
		{
			name: "strings with inline HIL interpolation of always-multiline objects are reformatted",
			expr: `"test${{ a = 1, b = 2}["a"]}"`,
			want: `"test${ {
				a = 1
				b = 2
			  }["a"]}"`,
		},
		{
			name: "multiline string inside HIL interpolation",
			expr: `"test${"something\nelse"}"`,
			want: `"testsomething\nelse"`,
		},
		{
			name: "strings ending with nl and interpolations returns heredocs",
			expr: `"${a}\ntest\n${global.a}\n"`,
			want: `<<-EOT
${a}
test
${global.a}
EOT
`,
		},
		{
			name: "strings with nl but not ending with nl returns plain string",
			expr: `"${a}\ntest\n${global.a}"`,
		},
		{
			name: "escaping carriege returns",
			expr: `"str \r str"`,
		},
		{
			name: "render string when generating heredoc",
			expr: `"\t${a}\n\ttest\n\t${global.a}\n"`,
			want: "<<-EOT\n\t${a}\n\ttest\n\t${global.a}\nEOT\n",
		},
		{
			name: "not render string when plain string",
			expr: `"\t${a}\n\ttest\n\t${global.a}"`,
		},
		{
			name: "render escape characters",
			expr: `"\n${a}${b}\t${b}\n\ntest\n\t${global.a}\n"`,
			want: "<<-EOT\n\n${a}${b}\t${b}\n\ntest\n\t${global.a}\nEOT\n",
		},
		{
			name: "render quotes",
			expr: `"this is a \"test\" string"`,
		},
		{
			name: "render quotes in heredocs",
			expr: `<<-EOT
this is a "test" string
EOT
`,
		},
		{
			name: "render escaped quotes in heredocs",
			expr: `<<-EOT
this is a \"test\" string
EOT
`,
		},
		{
			name: "utf-8",
			expr: `"伊亜希"`,
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
			name: "funcall with ellipsis",
			expr: `test([]...)`,
		},
		{
			name: "funcall with ellipsis after args",
			expr: `test(2, 3, 4, [1, 2, 3]...)`,
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
			name: "arithmetic binary operation (-) - sticky sign",
			expr: `A -A`,
		},
		{
			name: "arithmetic binary operation (+) - sticky plus",
			expr: `A +A`,
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
			name: "for-expr - object",
			expr: `{for k in c : k => k}`,
		},
		{
			name: "for-expr - object with ellipsis",
			expr: `{for i, v in ["a", "a", "b"]: v => i...}`,
		},
		{
			name: "for-expr - object with exprs and cond",
			expr: `{for k,v in expr() : expr()+test() => expr()+test()+1 if 0==0}`,
		},
		{
			name: "obj in string",
			expr: `"a ${{a = 1}} b"`,
			want: `"a ${{
				a = 1
			}} b"`,
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
			want := tc.want
			if want == "" {
				want = tc.expr
			}
			fmtWant := string(hclwrite.Format([]byte(want)))
			fmtGot := string(hclwrite.Format(got.Bytes()))
			for _, problem := range deep.Equal(fmtWant, fmtGot) {
				t.Errorf("problem: %s", problem)
			}
		})
	}
}

func BenchmarkTokensForExpression(b *testing.B) {
	exprStr := `[
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
	]`

	expr, diags := hclsyntax.ParseExpression([]byte(exprStr), "test.hcl", hcl.InitialPos)
	if diags.HasErrors() {
		panic(diags.Error())
	}
	for n := 0; n < b.N; n++ {
		ast.TokensForExpression(expr)
	}
}
