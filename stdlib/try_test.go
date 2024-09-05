// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stdlib_test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/stdlib"
	"github.com/terramate-io/terramate/test"
)

func TestTmTry(t *testing.T) {
	dir := test.TempDir(t)
	funcs := stdlib.Functions(dir, []string{})

	t.Run("single value number", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_try(1)`, "test.tm")
		assert.NoError(t, err)
		val, err := evalctx.Eval(expr)
		assert.NoError(t, err)
		if got, _ := val.AsBigFloat().Int64(); got != 1 {
			t.Fatalf("unexpected try result: %d", got)
		}
	})

	t.Run("single value string", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_try("hello")`, "test.tm")
		assert.NoError(t, err)
		val, err := evalctx.Eval(expr)
		assert.NoError(t, err)
		assert.EqualStrings(t, "hello", val.AsString())
	})

	t.Run("multiple working values, give back first successful", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_try("hello", "world")`, "test.tm")
		assert.NoError(t, err)
		val, err := evalctx.Eval(expr)
		assert.NoError(t, err)
		assert.EqualStrings(t, "hello", val.AsString())
	})

	t.Run("fallback not used", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_try("good world", "evil world")`, "test.tm")
		assert.NoError(t, err)
		val, err := evalctx.Eval(expr)
		assert.NoError(t, err)
		assert.EqualStrings(t, "good world", val.AsString())
	})

	t.Run("all arguments checked before fallback", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_try(unknown(), not.working, "works", "evil world")`, "test.tm")
		assert.NoError(t, err)
		val, err := evalctx.Eval(expr)
		assert.NoError(t, err)
		assert.EqualStrings(t, "works", val.AsString())
	})

	t.Run("fallback returned", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_try(unknown(), not.working, {}*1, "hello world")`, "test.tm")
		assert.NoError(t, err)
		val, err := evalctx.Eval(expr)
		assert.NoError(t, err)
		assert.EqualStrings(t, "hello world", val.AsString())
	})

	t.Run("unknowns in expr works with fallback", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_try({a = some.thing}, "world")`, "test.tm")
		assert.NoError(t, err)
		val, err := evalctx.Eval(expr)
		assert.NoError(t, err)
		assert.EqualStrings(t, "world", val.AsString())
	})

	t.Run("funcall with unknowns in expr works with fallback", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_try(tm_upper(some.unknown), "world")`, "test.tm")
		assert.NoError(t, err)
		val, err := evalctx.Eval(expr)
		assert.NoError(t, err)
		assert.EqualStrings(t, "world", val.AsString())
	})
}
