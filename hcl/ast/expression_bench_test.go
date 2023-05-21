// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package ast_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/terramate-io/terramate/hcl/ast"
)

var printTokens = false

func BenchmarkTokensForExpressionComplex(b *testing.B) {
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
	var tokens hclwrite.Tokens
	for n := 0; n < b.N; n++ {
		tokens = ast.TokensForExpression(expr)
	}
	if printTokens {
		fmt.Println(string(tokens.Bytes()))
	}
}

func BenchmarkTokensForExpressionPlainStringNoNewline(b *testing.B) {
	exprStr := `"plain string"`
	expr, diags := hclsyntax.ParseExpression([]byte(exprStr), "test.hcl", hcl.InitialPos)
	if diags.HasErrors() {
		panic(diags.Error())
	}
	var tokens hclwrite.Tokens
	for n := 0; n < b.N; n++ {
		tokens = ast.TokensForExpression(expr)
	}
	if printTokens {
		fmt.Println(string(tokens.Bytes()))
	}
}

func BenchmarkTokensForExpressionStringWith100Newlines(b *testing.B) {
	repeat := `plain string\n`
	exprStr := `"` + strings.Repeat(repeat, 100) + `"`
	expr, diags := hclsyntax.ParseExpression([]byte(exprStr), "test.hcl", hcl.InitialPos)
	if diags.HasErrors() {
		panic(diags.Error())
	}
	var tokens hclwrite.Tokens
	for n := 0; n < b.N; n++ {
		tokens = ast.TokensForExpression(expr)
	}
	if printTokens {
		fmt.Println(string(tokens.Bytes()))
	}
}

func BenchmarkTokensForExpressionObjectWith100KeysWithNumberValues(b *testing.B) {
	var exprStr = `{`
	for i := 0; i < 100; i++ {
		exprStr += fmt.Sprintf("\tkey%d = %d\n", i, i)
	}
	exprStr += "}"
	expr, diags := hclsyntax.ParseExpression([]byte(exprStr), "test.hcl", hcl.InitialPos)
	if diags.HasErrors() {
		panic(diags.Error())
	}
	var tokens hclwrite.Tokens
	for n := 0; n < b.N; n++ {
		tokens = ast.TokensForExpression(expr)
	}
	if printTokens {
		fmt.Println(string(tokens.Bytes()))
	}
}
