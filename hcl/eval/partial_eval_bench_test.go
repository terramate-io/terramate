// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package eval_test

import (
	"os"
	"strings"
	"testing"

	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/terramate-io/terramate/globals"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/hcl/info"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/stdlib"
	"github.com/terramate-io/terramate/test/sandbox"
	"github.com/zclconf/go-cty/cty"
)

func setupContext(b *testing.B) *eval.Context {
	s := sandbox.New(b)
	builtinInfo := eval.Info{
		Scope: project.NewPath("/"),
		DefinedAt: info.NewRange(s.RootDir(), hhcl.Range{
			Start: hhcl.InitialPos,
			End:   hhcl.InitialPos,
		}),
	}
	ctx := eval.New(
		s.Config().Tree().Dir(),
		globals.NewResolver(
			s.Config(),
			eval.NewValStmt(eval.NewRef("global", "true"), cty.True, builtinInfo),
			eval.NewValStmt(eval.NewRef("global", "false"), cty.False, builtinInfo),
			eval.NewValStmt(eval.NewRef("global", "number"), cty.NumberFloatVal(3.141516), builtinInfo),
			eval.NewValStmt(eval.NewRef("global", "string"), cty.StringVal("terramate"), builtinInfo),
			eval.NewValStmt(eval.NewRef("global", "list"), cty.ListVal([]cty.Value{
				cty.NumberIntVal(0),
				cty.NumberIntVal(1),
				cty.NumberIntVal(2),
				cty.NumberIntVal(3),
			}), builtinInfo),
			eval.NewValStmt(eval.NewRef("global", "strings"), cty.ListVal([]cty.Value{
				cty.StringVal("terramate"),
				cty.StringVal("is"),
				cty.StringVal("fun"),
			}), builtinInfo),
			eval.NewValStmt(eval.NewRef("global", "obj"), cty.ObjectVal(map[string]cty.Value{
				"a": cty.NumberIntVal(0),
				"b": cty.ListVal([]cty.Value{cty.StringVal("terramate")}),
			}), builtinInfo),
		))
	ctx.SetFunctions(stdlib.Functions(ctx, os.TempDir()))
	return ctx
}

func BenchmarkPartialEvalComplex(b *testing.B) {
	b.StopTimer()
	ctx := setupContext(b)

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
		expr, diags := hclsyntax.ParseExpression(exprBytes, "<bench>", hhcl.InitialPos)
		if diags.HasErrors() {
			b.Fatalf(diags.Error())
		}
		_, err := ctx.PartialEval(expr)
		if err != nil {
			b.Fatal(err.Error())
		}
	}
}

func BenchmarkPartialEvalSmallString(b *testing.B) {
	b.StopTimer()
	ctx := setupContext(b)

	exprBytes := []byte(`"terramate is fun"`)

	b.StartTimer()
	for n := 0; n < b.N; n++ {
		expr, diags := hclsyntax.ParseExpression(exprBytes, "<bench>", hhcl.InitialPos)
		if diags.HasErrors() {
			b.Fatalf(diags.Error())
		}
		_, err := ctx.PartialEval(expr)
		if err != nil {
			b.Fatal(err.Error())
		}
	}
}

func BenchmarkPartialEvalHugeString(b *testing.B) {
	b.StopTimer()
	ctx := setupContext(b)

	exprBytes := []byte(`"` + strings.Repeat(`terramate is fun\n`, 1000) + `"`)

	b.StartTimer()
	for n := 0; n < b.N; n++ {
		expr, diags := hclsyntax.ParseExpression(exprBytes, "<bench>", hhcl.InitialPos)
		if diags.HasErrors() {
			b.Fatalf(diags.Error())
		}
		_, err := ctx.PartialEval(expr)
		if err != nil {
			b.Fatal(err.Error())
		}
	}
}

func BenchmarkPartialEvalHugeInterpolatedString(b *testing.B) {
	b.StopTimer()
	ctx := setupContext(b)

	exprBytes := []byte(`"` + strings.Repeat(`${global.string} is fun\n`, 1000) + `"`)

	b.StartTimer()
	for n := 0; n < b.N; n++ {
		expr, diags := hclsyntax.ParseExpression(exprBytes, "<bench>", hhcl.InitialPos)
		if diags.HasErrors() {
			b.Fatalf(diags.Error())
		}
		_, err := ctx.PartialEval(expr)
		if err != nil {
			b.Fatal(err.Error())
		}
	}
}

func BenchmarkPartialEvalObject(b *testing.B) {
	b.StopTimer()
	ctx := setupContext(b)

	exprBytes := []byte(`{
		a = 1
		b = [0, 1, 2, 3]
		c = [global.number, global.string]
		d = [global.list]	
	}`)

	b.StartTimer()
	for n := 0; n < b.N; n++ {
		expr, diags := hclsyntax.ParseExpression(exprBytes, "<bench>", hhcl.InitialPos)
		if diags.HasErrors() {
			b.Fatalf(diags.Error())
		}
		_, err := ctx.PartialEval(expr)
		if err != nil {
			b.Fatal(err.Error())
		}
	}
}
