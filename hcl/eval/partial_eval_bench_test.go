// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package eval_test

import (
	"os"
	"strings"
	"testing"

	"github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/hcl/v2/hclsyntax"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/stdlib"
	"github.com/zclconf/go-cty/cty"
)

func setupContext() *eval.Context {
	ctx := eval.NewContext(stdlib.Functions(os.TempDir(), []string{}))
	ctx.SetNamespace("global", map[string]cty.Value{
		"true":   cty.True,
		"false":  cty.False,
		"number": cty.NumberFloatVal(3.141516),
		"string": cty.StringVal("terramate"),
		"list": cty.ListVal([]cty.Value{
			cty.NumberIntVal(0),
			cty.NumberIntVal(1),
			cty.NumberIntVal(2),
			cty.NumberIntVal(3),
		}),
		"strings": cty.ListVal([]cty.Value{
			cty.StringVal("terramate"),
			cty.StringVal("is"),
			cty.StringVal("fun"),
		}),
		"obj": cty.ObjectVal(map[string]cty.Value{
			"a": cty.NumberIntVal(0),
			"b": cty.ListVal([]cty.Value{cty.StringVal("terramate")}),
		}),
	})
	return ctx
}

func BenchmarkPartialEvalComplex(b *testing.B) {
	b.StopTimer()
	ctx := setupContext()

	exprBytes := []byte(`[
		{
			a = "prefix ${tm_upper(global.string)} ${global.number} suffix"
			b = [0, 1, global.true, global.false, global.number, global.string, global.list, global.obj]
			c = {
				a = tm_floor(global.number) == 3 ? tm_upper(global.string) : tm_title(global.string)
				b = 10*global.number+global.number / 2+3
			}
			e = tm_concat(global.list, [tm_max(21, 8, 13, 3, 1, 5, 1, 2)])
		},
		{
			a = "prefix ${tm_upper(global.string)} ${global.number} suffix"
			b = [0, 1, global.true, global.false, global.number, global.string, global.list, global.obj]
			c = {
				a = tm_floor(global.number) == 3 ? tm_upper(global.string) : tm_title(global.string)
				b = 10*global.number+global.number / 2+3
			}
			e = tm_concat(global.list, [tm_max(21, 8, 13, 3, 1, 5, 1, 2)])
		},
		{
			a = "prefix ${tm_upper(global.string)} ${global.number} suffix"
			b = [0, 1, global.true, global.false, global.number, global.string, global.list, global.obj]
			c = {
				a = tm_floor(global.number) == 3 ? tm_upper(global.string) : tm_title(global.string)
				b = 10*global.number+global.number / 2+3
			}
			e = tm_concat(global.list, [tm_max(21, 8, 13, 3, 1, 5, 1, 2)])
		},
		{
			a = "prefix ${tm_upper(global.string)} ${global.number} suffix"
			b = [0, 1, global.true, global.false, global.number, global.string, global.list, global.obj]
			c = {
				a = tm_floor(global.number) == 3 ? tm_upper(global.string) : tm_title(global.string)
				b = 10*global.number+global.number / 2+3
			}
			e = tm_concat(global.list, [tm_max(21, 8, 13, 3, 1, 5, 1, 2)])
		},
		{
			a = "prefix ${tm_upper(global.string)} ${global.number} suffix"
			b = [0, 1, global.true, global.false, global.number, global.string, global.list, global.obj]
			c = {
				a = tm_floor(global.number) == 3 ? tm_upper(global.string) : tm_title(global.string)
				b = 10*global.number+global.number / 2+3
			}
			e = tm_concat(global.list, [tm_max(21, 8, 13, 3, 1, 5, 1, 2)])
		},
	]`)

	b.StartTimer()
	for n := 0; n < b.N; n++ {
		expr, diags := hclsyntax.ParseExpression(exprBytes, "<bench>", hcl.InitialPos)
		if diags.HasErrors() {
			b.Fatal(diags.Error())
		}
		_, _, err := ctx.PartialEval(expr)
		if err != nil {
			b.Fatal(err.Error())
		}
	}
}

func BenchmarkPartialEvalSmallString(b *testing.B) {
	b.StopTimer()
	ctx := setupContext()

	exprBytes := []byte(`"terramate is fun"`)

	b.StartTimer()
	for n := 0; n < b.N; n++ {
		expr, diags := hclsyntax.ParseExpression(exprBytes, "<bench>", hcl.InitialPos)
		if diags.HasErrors() {
			b.Fatal(diags.Error())
		}
		_, _, err := ctx.PartialEval(expr)
		if err != nil {
			b.Fatal(err.Error())
		}
	}
}

func BenchmarkPartialEvalHugeString(b *testing.B) {
	b.StopTimer()
	ctx := setupContext()

	exprBytes := []byte(`"` + strings.Repeat(`terramate is fun\n`, 1000) + `"`)

	b.StartTimer()
	for n := 0; n < b.N; n++ {
		expr, diags := hclsyntax.ParseExpression(exprBytes, "<bench>", hcl.InitialPos)
		if diags.HasErrors() {
			b.Fatal(diags.Error())
		}
		_, _, err := ctx.PartialEval(expr)
		if err != nil {
			b.Fatal(err.Error())
		}
	}
}

func BenchmarkPartialEvalHugeInterpolatedString(b *testing.B) {
	b.StopTimer()
	ctx := setupContext()

	exprBytes := []byte(`"` + strings.Repeat(`${global.string} is fun\n`, 1000) + `"`)

	b.StartTimer()
	for n := 0; n < b.N; n++ {
		expr, diags := hclsyntax.ParseExpression(exprBytes, "<bench>", hcl.InitialPos)
		if diags.HasErrors() {
			b.Fatal(diags.Error())
		}
		_, _, err := ctx.PartialEval(expr)
		if err != nil {
			b.Fatal(err.Error())
		}
	}
}

func BenchmarkPartialEvalObject(b *testing.B) {
	b.StopTimer()
	ctx := setupContext()

	exprBytes := []byte(`{
		a = 1
		b = [0, 1, 2, 3]
		c = [global.number, global.string]
		d = [global.list]	
	}`)

	b.StartTimer()
	for n := 0; n < b.N; n++ {
		expr, diags := hclsyntax.ParseExpression(exprBytes, "<bench>", hcl.InitialPos)
		if diags.HasErrors() {
			b.Fatal(diags.Error())
		}
		_, _, err := ctx.PartialEval(expr)
		if err != nil {
			b.Fatal(err.Error())
		}
	}
}
