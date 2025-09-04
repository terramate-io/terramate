// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stdlib_test

import (
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/stdlib"
	"github.com/terramate-io/terramate/test"
	"github.com/zclconf/go-cty/cty"
)

func TestTmJoinList(t *testing.T) {
	dir := test.TempDir(t)
	funcs := stdlib.Functions(dir, []string{})

	t.Run("basic join - single elements", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_joinlist("-", [["a"], ["b"], ["c"]])`, "test.tm")
		assert.NoError(t, err)
		val, err := evalctx.Eval(expr)
		assert.NoError(t, err)

		expected := []string{"a", "b", "c"}
		result := extractStringList(t, val)
		assert.EqualInts(t, len(expected), len(result))
		for i, exp := range expected {
			assert.EqualStrings(t, exp, result[i])
		}
	})

	t.Run("basic join - multiple elements", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_joinlist("-", [["a"], ["a","b"], ["a","b","c"]])`, "test.tm")
		assert.NoError(t, err)
		val, err := evalctx.Eval(expr)
		assert.NoError(t, err)

		expected := []string{"a", "a-b", "a-b-c"}
		result := extractStringList(t, val)
		assert.EqualInts(t, len(expected), len(result))
		for i, exp := range expected {
			assert.EqualStrings(t, exp, result[i])
		}
	})

	t.Run("slash separator for paths", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_joinlist("/", [["root"], ["root", "child"], ["root", "child", "leaf"]])`, "test.tm")
		assert.NoError(t, err)
		val, err := evalctx.Eval(expr)
		assert.NoError(t, err)

		expected := []string{"root", "root/child", "root/child/leaf"}
		result := extractStringList(t, val)
		assert.EqualInts(t, len(expected), len(result))
		for i, exp := range expected {
			assert.EqualStrings(t, exp, result[i])
		}
	})

	t.Run("empty input list", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_joinlist("/", [])`, "test.tm")
		assert.NoError(t, err)
		val, err := evalctx.Eval(expr)
		assert.NoError(t, err)

		result := extractStringList(t, val)
		assert.EqualInts(t, 0, len(result))
	})

	t.Run("empty sublist", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_joinlist("/", [["a"], []])`, "test.tm")
		assert.NoError(t, err)
		val, err := evalctx.Eval(expr)
		assert.NoError(t, err)

		expected := []string{"a", ""}
		result := extractStringList(t, val)
		assert.EqualInts(t, len(expected), len(result))
		for i, exp := range expected {
			assert.EqualStrings(t, exp, result[i])
		}
	})

	t.Run("complex tree-like structure", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_joinlist("/", [
			["Grand Parent A"], 
			["Grand Parent A", "Parent A"], 
			["Grand Parent A", "Parent A", "Child A"], 
			["Grand Parent A", "Parent A", "Child B"]
		])`, "test.tm")
		assert.NoError(t, err)
		val, err := evalctx.Eval(expr)
		assert.NoError(t, err)

		expected := []string{
			"Grand Parent A",
			"Grand Parent A/Parent A",
			"Grand Parent A/Parent A/Child A",
			"Grand Parent A/Parent A/Child B",
		}
		result := extractStringList(t, val)
		assert.EqualInts(t, len(expected), len(result))
		for i, exp := range expected {
			assert.EqualStrings(t, exp, result[i])
		}
	})

	t.Run("single character separator", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_joinlist("|", [["x", "y", "z"]])`, "test.tm")
		assert.NoError(t, err)
		val, err := evalctx.Eval(expr)
		assert.NoError(t, err)

		expected := []string{"x|y|z"}
		result := extractStringList(t, val)
		assert.EqualInts(t, len(expected), len(result))
		for i, exp := range expected {
			assert.EqualStrings(t, exp, result[i])
		}
	})

	t.Run("multi-character separator", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_joinlist(" :: ", [["namespace", "module", "function"]])`, "test.tm")
		assert.NoError(t, err)
		val, err := evalctx.Eval(expr)
		assert.NoError(t, err)

		expected := []string{"namespace :: module :: function"}
		result := extractStringList(t, val)
		assert.EqualInts(t, len(expected), len(result))
		for i, exp := range expected {
			assert.EqualStrings(t, exp, result[i])
		}
	})

	t.Run("empty string separator", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_joinlist("", [["a", "b", "c"]])`, "test.tm")
		assert.NoError(t, err)
		val, err := evalctx.Eval(expr)
		assert.NoError(t, err)

		expected := []string{"abc"}
		result := extractStringList(t, val)
		assert.EqualInts(t, len(expected), len(result))
		for i, exp := range expected {
			assert.EqualStrings(t, exp, result[i])
		}
	})
}

func TestTmJoinListErrors(t *testing.T) {
	dir := test.TempDir(t)
	funcs := stdlib.Functions(dir, []string{})

	t.Run("wrong element type - number instead of list", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_joinlist("/", [42])`, "test.tm")
		assert.NoError(t, err)
		_, err = evalctx.Eval(expr)
		assert.Error(t, err)
		assert.IsTrue(t, strings.Contains(err.Error(), "tm_joinlist: expected all elements to be list(string), got number at index 0"))
	})

	t.Run("wrong nested element type - list(number) instead of list(string)", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_joinlist("/", [[1, 2]])`, "test.tm")
		assert.NoError(t, err)
		_, err = evalctx.Eval(expr)
		assert.Error(t, err)
		// HCL parses [1, 2] as tuple(number, number), so the error message will be different
		assert.IsTrue(t, strings.Contains(err.Error(), "tm_joinlist: expected all elements to be list(string)") &&
			(strings.Contains(err.Error(), "tuple with number") || strings.Contains(err.Error(), "list(number)")))
	})

	t.Run("mixed types in list", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_joinlist("/", [["valid"], "invalid"])`, "test.tm")
		assert.NoError(t, err)
		_, err = evalctx.Eval(expr)
		assert.Error(t, err)
		assert.IsTrue(t, strings.Contains(err.Error(), "tm_joinlist: expected all elements to be list(string), got string at index 1"))
	})

	t.Run("wrong element type at different index", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_joinlist("/", [["valid1"], ["valid2"], ["valid3"], true])`, "test.tm")
		assert.NoError(t, err)
		_, err = evalctx.Eval(expr)
		assert.Error(t, err)
		assert.IsTrue(t, strings.Contains(err.Error(), "tm_joinlist: expected all elements to be list(string), got bool at index 3"))
	})
}

func TestTmJoinListDeterminism(t *testing.T) {
	dir := test.TempDir(t)
	funcs := stdlib.Functions(dir, []string{})

	// Test that the function produces identical results for identical inputs
	evalctx := eval.NewContext(funcs)
	expr, err := ast.ParseExpression(`tm_joinlist("/", [["a", "b"], ["c", "d"]])`, "test.tm")
	assert.NoError(t, err)

	// Call the function multiple times
	var results [][]string
	for i := 0; i < 5; i++ {
		val, err := evalctx.Eval(expr)
		assert.NoError(t, err)
		result := extractStringList(t, val)
		results = append(results, result)
	}

	// Verify all results are identical
	for i := 1; i < len(results); i++ {
		assert.EqualInts(t, len(results[0]), len(results[i]))
		for j, exp := range results[0] {
			assert.EqualStrings(t, exp, results[i][j])
		}
	}
}

func TestTmJoinListCustomErrorTypes(t *testing.T) {
	t.Parallel()

	// Test that we get the specific custom error types
	fn := stdlib.JoinListFunc()

	t.Run("JoinListInvalidArgumentTypeError", func(t *testing.T) {
		_, err := fn.Call([]cty.Value{
			cty.StringVal("/"),
			cty.StringVal("not a list"),
		})

		assert.IsTrue(t, errors.IsKind(err, stdlib.JoinListInvalidArgumentType))
		assert.IsTrue(t, strings.Contains(err.Error(), "string"))
	})

	t.Run("JoinListInvalidElementTypeError", func(t *testing.T) {
		_, err := fn.Call([]cty.Value{
			cty.StringVal("/"),
			cty.ListVal([]cty.Value{
				cty.NumberIntVal(42),
			}),
		})

		assert.IsTrue(t, errors.IsKind(err, stdlib.JoinListInvalidElementType))
		assert.IsTrue(t, strings.Contains(err.Error(), "number"))
		assert.IsTrue(t, strings.Contains(err.Error(), "index 0"))
	})

	t.Run("JoinListInvalidListElementTypeError", func(t *testing.T) {
		_, err := fn.Call([]cty.Value{
			cty.StringVal("/"),
			cty.ListVal([]cty.Value{
				cty.ListVal([]cty.Value{cty.NumberIntVal(1), cty.NumberIntVal(2)}),
			}),
		})

		assert.IsTrue(t, errors.IsKind(err, stdlib.JoinListInvalidListElementType))
		assert.IsTrue(t, strings.Contains(err.Error(), "number"))
		assert.IsTrue(t, strings.Contains(err.Error(), "index 0"))
	})

	t.Run("JoinListInvalidTupleElementTypeError", func(t *testing.T) {
		_, err := fn.Call([]cty.Value{
			cty.StringVal("/"),
			cty.TupleVal([]cty.Value{
				cty.TupleVal([]cty.Value{cty.StringVal("valid"), cty.NumberIntVal(42)}),
			}),
		})

		assert.IsTrue(t, errors.IsKind(err, stdlib.JoinListInvalidTupleElementType))
		assert.IsTrue(t, strings.Contains(err.Error(), "number"))
		assert.IsTrue(t, strings.Contains(err.Error(), "element 1"))
		assert.IsTrue(t, strings.Contains(err.Error(), "index 0"))
	})
}

func TestTmJoinListOrderPreservation(t *testing.T) {
	dir := test.TempDir(t)
	funcs := stdlib.Functions(dir, []string{})

	evalctx := eval.NewContext(funcs)
	expr, err := ast.ParseExpression(`tm_joinlist("|", [
		["first", "element"], 
		["second", "element"], 
		["third", "element"]
	])`, "test.tm")
	assert.NoError(t, err)
	val, err := evalctx.Eval(expr)
	assert.NoError(t, err)

	expected := []string{
		"first|element",
		"second|element",
		"third|element",
	}
	result := extractStringList(t, val)
	assert.EqualInts(t, len(expected), len(result))
	for i, exp := range expected {
		assert.EqualStrings(t, exp, result[i])
	}
}

// Helper function to extract string list from cty.Value
func extractStringList(t *testing.T, val cty.Value) []string {
	t.Helper()

	if !val.Type().IsListType() {
		t.Fatalf("expected list type, got %s", val.Type().FriendlyName())
	}

	var result []string
	it := val.ElementIterator()
	for it.Next() {
		_, element := it.Element()
		if !element.Type().Equals(cty.String) {
			t.Fatalf("expected string element, got %s", element.Type().FriendlyName())
		}
		result = append(result, element.AsString())
	}

	return result
}
