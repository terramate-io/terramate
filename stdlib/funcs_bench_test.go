// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stdlib_test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/stdlib"
	"github.com/terramate-io/terramate/test"
	"github.com/zclconf/go-cty/cty"
)

func BenchmarkTmAllTrueLiteralList(b *testing.B) {
	b.StopTimer()
	evalctx := eval.NewContext(stdlib.Functions(test.TempDir(b)))
	expr, err := ast.ParseExpression(`tm_alltrue([
		false,
		tm_element(tm_range(0, 100), 0) == 0,
		tm_length(tm_distinct([for i in tm_range(0, 100): 0*i]))==1,
	])`, `bench-test`)
	assert.NoError(b, err)
	b.StartTimer()
	for n := 0; n < b.N; n++ {
		v, err := evalctx.Eval(expr)
		assert.NoError(b, err)
		if got := v.True(); got {
			b.Fatalf("unexpected value: %t", got)
		}
	}
}

func BenchmarkTmAllTrueFuncall(b *testing.B) {
	b.StopTimer()
	evalctx := eval.NewContext(stdlib.Functions(test.TempDir(b)))
	expr, err := ast.ParseExpression(`tm_alltrue(tm_distinct([for i in tm_range(0, 3) : i == 2 ? true : false]))`, `bench-test`)
	assert.NoError(b, err)
	b.StartTimer()
	for n := 0; n < b.N; n++ {
		v, err := evalctx.Eval(expr)
		assert.NoError(b, err)
		if got := v.True(); got {
			b.Fatalf("unexpected value: %t", got)
		}
	}
}

func BenchmarkTmAnyTrueLiteralList(b *testing.B) {
	b.StopTimer()
	evalctx := eval.NewContext(stdlib.Functions(test.TempDir(b)))
	expr, err := ast.ParseExpression(`tm_anytrue([
		true,
		tm_element(tm_range(0, 100), 0) != 0,
		tm_length(tm_distinct([for i in tm_range(0, 100): 2*i]))>1,
	])`, `bench-test`)
	assert.NoError(b, err)
	b.StartTimer()
	for n := 0; n < b.N; n++ {
		v, err := evalctx.Eval(expr)
		assert.NoError(b, err)
		if got := v.True(); !got {
			b.Fatalf("unexpected value: %t", got)
		}
	}
}

func BenchmarkTmAnyTrueFuncall(b *testing.B) {
	b.StopTimer()
	evalctx := eval.NewContext(stdlib.Functions(test.TempDir(b)))
	expr, err := ast.ParseExpression(`tm_anytrue(tm_distinct([for i in tm_range(0, 3) : i == 2 ? true : false]))`, `bench-test`)
	assert.NoError(b, err)
	b.StartTimer()
	for n := 0; n < b.N; n++ {
		v, err := evalctx.Eval(expr)
		assert.NoError(b, err)
		if got := v.True(); !got {
			b.Fatalf("unexpected value: %t", got)
		}
	}
}

func BenchmarkTmTernary(b *testing.B) {
	b.StopTimer()
	evalctx := eval.NewContext(stdlib.Functions(test.TempDir(b)))
	expr, err := ast.ParseExpression(`tm_ternary(false, tm_unknown_function(), "result")`, `bench-test`)
	assert.NoError(b, err)
	b.StartTimer()
	for n := 0; n < b.N; n++ {
		v, err := evalctx.Eval(expr)
		assert.NoError(b, err)
		if got := v.AsString(); got != "result" {
			b.Fatalf("unexpected value: %s", got)
		}
	}
}

func BenchmarkTmTryUnknownFunc(b *testing.B) {
	b.StopTimer()
	evalctx := eval.NewContext(stdlib.Functions(test.TempDir(b)))
	expr, err := ast.ParseExpression(`tm_try(tm_unknown_function(), "result")`, `bench-test`)
	assert.NoError(b, err)
	b.StartTimer()
	for n := 0; n < b.N; n++ {
		v, err := evalctx.Eval(expr)
		assert.NoError(b, err)
		if got := v.AsString(); got != "result" {
			b.Fatalf("unexpected value: %s", got)
		}
	}
}

func BenchmarkTmTryUnknownVariable(b *testing.B) {
	b.StopTimer()
	evalctx := eval.NewContext(stdlib.Functions(test.TempDir(b)))
	expr, err := ast.ParseExpression(`tm_try(unknown_variable, "result")`, `bench-test`)
	assert.NoError(b, err)
	b.StartTimer()
	for n := 0; n < b.N; n++ {
		v, err := evalctx.Eval(expr)
		assert.NoError(b, err)
		if got := v.AsString(); got != "result" {
			b.Fatalf("unexpected value: %s", got)
		}
	}
}

func BenchmarkTmTryUnknownObjectKey(b *testing.B) {
	b.StopTimer()
	evalctx := eval.NewContext(stdlib.Functions(test.TempDir(b)))
	evalctx.SetNamespaceRaw("let", cty.ObjectVal(map[string]cty.Value{
		"some": cty.ObjectVal(map[string]cty.Value{}),
	}))
	expr, err := ast.ParseExpression(`tm_try(let.some.unknown, "result")`, `bench-test`)
	assert.NoError(b, err)
	b.StartTimer()
	for n := 0; n < b.N; n++ {
		v, err := evalctx.Eval(expr)
		assert.NoError(b, err)
		if got := v.AsString(); got != "result" {
			b.Fatalf("unexpected value: %s", got)
		}
	}
}
