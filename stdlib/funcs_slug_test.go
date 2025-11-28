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

// TestSlugFunctionIntegration tests the function through the evaluation context
func TestSlugFunctionIntegration(t *testing.T) {
	slugFunc := stdlib.SlugFunc()

	tests := []struct {
		name        string
		input       []cty.Value
		expected    cty.Value
		expectError bool
	}{
		{
			"string input",
			[]cty.Value{cty.StringVal("Hello World!")},
			cty.StringVal("hello-world-"),
			false,
		},
		{
			"list input",
			[]cty.Value{cty.ListVal([]cty.Value{
				cty.StringVal("A"),
				cty.StringVal("B C"),
			})},
			cty.ListVal([]cty.Value{
				cty.StringVal("a"),
				cty.StringVal("b-c"),
			}),
			false,
		},
		{
			"empty list",
			[]cty.Value{cty.ListValEmpty(cty.String)},
			cty.ListValEmpty(cty.String),
			false,
		},
		{
			"invalid type",
			[]cty.Value{cty.NumberIntVal(123)},
			cty.NilVal,
			true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Test implementation
			result, err := slugFunc.Call(tc.input)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.IsTrue(t, tc.expected.RawEquals(result))
			}
		})
	}
}

// Benchmark test for performance validation
func BenchmarkSlugifyLargeList(b *testing.B) {
	// Create a list with 1000 elements
	elements := make([]cty.Value, 1000)
	for i := 0; i < 1000; i++ {
		elements[i] = cty.StringVal("Test String With Spaces And Special Characters!")
	}
	input := []cty.Value{cty.ListVal(elements)}

	slugFunc := stdlib.SlugFunc()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := slugFunc.Call(input)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// TestTmSlugIntegrationWithEval tests the function through the evaluation context
func TestTmSlugIntegrationWithEval(t *testing.T) {
	t.Parallel()

	type testcase struct {
		expr string
		want cty.Value
	}

	tests := []testcase{
		{
			expr: `tm_slug("I am the slug")`,
			want: cty.StringVal("i-am-the-slug"),
		},
		{
			expr: `tm_slug("i-am-already-slug-1")`,
			want: cty.StringVal("i-am-already-slug-1"),
		},
		{
			expr: `tm_slug("Dollar$%special")`,
			want: cty.StringVal("dollar--special"),
		},
		{
			expr: `tm_slug("")`,
			want: cty.StringVal(""),
		},
		{
			expr: `tm_slug(["Hello World!", "Grand/Child A"])`,
			want: cty.ListVal([]cty.Value{
				cty.StringVal("hello-world-"),
				cty.StringVal("grand-child-a"),
			}),
		},
		{
			expr: `tm_slug([])`,
			want: cty.ListValEmpty(cty.String),
		},
		{
			expr: `tm_slug(["café", "naïve"])`,
			want: cty.ListVal([]cty.Value{
				cty.StringVal("caf-"),
				cty.StringVal("na-ve"),
			}),
		},
		{
			expr: `tm_slug(["A","B C","D/E"])`,
			want: cty.ListVal([]cty.Value{
				cty.StringVal("a"),
				cty.StringVal("b-c"),
				cty.StringVal("d-e"),
			}),
		},
		{
			expr: `tm_slug(null)`,
			want: cty.DynamicVal,
		},
		{
			expr: `{a = "hello", b = tm_slug(null)}`,
			want: cty.ObjectVal(map[string]cty.Value{
				"a": cty.StringVal("hello"),
				"b": cty.DynamicVal,
			}),
		},
	}

	for _, tc := range tests {
		t.Run(tc.expr, func(t *testing.T) {
			rootdir := test.TempDir(t)
			ctx := eval.NewContext(stdlib.Functions(rootdir, []string{}))
			val, err := ctx.Eval(test.NewExpr(t, tc.expr))
			assert.NoError(t, err)
			assert.IsTrue(t, tc.want.RawEquals(val))
		})
	}
}

// TestTmSlugErrorsIntegrationWithEval tests error cases through the evaluation context
func TestTmSlugErrorsIntegrationWithEval(t *testing.T) {
	t.Parallel()

	errorTests := []struct {
		expr        string
		expectError string
	}{
		{
			expr:        `tm_slug(123)`,
			expectError: "tm_slug: expected string or list(string), got number",
		},
		{
			expr:        `tm_slug(["A", "B", 123])`,
			expectError: "tm_slug: list contains non-string element at index 2: number",
		},
	}

	for _, tc := range errorTests {
		t.Run(tc.expr, func(t *testing.T) {
			rootdir := test.TempDir(t)
			ctx := eval.NewContext(stdlib.Functions(rootdir, []string{}))
			_, err := ctx.Eval(test.NewExpr(t, tc.expr))
			assert.Error(t, err)
			if !strings.Contains(err.Error(), tc.expectError) {
				t.Logf("Expected error to contain %q, but got %q", tc.expectError, err.Error())
			}
			assert.IsTrue(t, strings.Contains(err.Error(), tc.expectError))
		})
	}
}

// TestTmSlugSpecificErrors tests the specific error kinds for tm_slug
func TestTmSlugSpecificErrors(t *testing.T) {
	// Test that we get the specific custom error types
	fn := stdlib.SlugFunc()

	t.Run("SlugWrongType", func(t *testing.T) {
		_, err := fn.Call([]cty.Value{
			cty.NumberIntVal(123),
		})

		assert.IsTrue(t, errors.IsKind(err, stdlib.SlugWrongType))
		assert.IsTrue(t, strings.Contains(err.Error(), "number"))
	})

	t.Run("SlugListElementNotString", func(t *testing.T) {
		_, err := fn.Call([]cty.Value{
			cty.TupleVal([]cty.Value{
				cty.StringVal("valid"),
				cty.NumberIntVal(42),
			}),
		})

		assert.IsTrue(t, errors.IsKind(err, stdlib.SlugListElementNotString))
		assert.IsTrue(t, strings.Contains(err.Error(), "index 1"))
		assert.IsTrue(t, strings.Contains(err.Error(), "number"))
	})

	t.Run("SlugListElementNotStringInTuple", func(t *testing.T) {
		_, err := fn.Call([]cty.Value{
			cty.TupleVal([]cty.Value{
				cty.StringVal("valid"),
				cty.BoolVal(true),
				cty.StringVal("another valid"),
			}),
		})

		assert.IsTrue(t, errors.IsKind(err, stdlib.SlugListElementNotString))
		assert.IsTrue(t, strings.Contains(err.Error(), "index 1"))
		assert.IsTrue(t, strings.Contains(err.Error(), "bool"))
	})
}

// TestTmSlugNullHandling tests null handling in lists and tuples
func TestTmSlugNullHandling(t *testing.T) {
	fn := stdlib.SlugFunc()

	t.Run("ListWithNullElements", func(t *testing.T) {
		result, err := fn.Call([]cty.Value{
			cty.ListVal([]cty.Value{
				cty.StringVal("Hello World!"),
				cty.NullVal(cty.String),
				cty.StringVal("Another Valid"),
			}),
		})

		// This should not panic but should handle null gracefully
		assert.NoError(t, err)
		assert.IsTrue(t, result.Type().IsListType())
		assert.EqualInts(t, 3, result.LengthInt())

		// Check individual elements
		it := result.ElementIterator()
		var elements []cty.Value
		for it.Next() {
			_, elem := it.Element()
			elements = append(elements, elem)
		}

		assert.EqualStrings(t, "hello-world-", elements[0].AsString())
		assert.IsTrue(t, elements[1].IsNull())
		assert.EqualStrings(t, "another-valid", elements[2].AsString())
	})

	t.Run("TupleWithNullElements", func(t *testing.T) {
		result, err := fn.Call([]cty.Value{
			cty.TupleVal([]cty.Value{
				cty.StringVal("Hello World!"),
				cty.NullVal(cty.String),
				cty.StringVal("Another Valid"),
			}),
		})

		// This should not panic but should handle null gracefully
		assert.NoError(t, err)
		assert.IsTrue(t, result.Type().IsListType())
		assert.EqualInts(t, 3, result.LengthInt())

		// Check individual elements
		it := result.ElementIterator()
		var elements []cty.Value
		for it.Next() {
			_, elem := it.Element()
			elements = append(elements, elem)
		}

		assert.EqualStrings(t, "hello-world-", elements[0].AsString())
		assert.IsTrue(t, elements[1].IsNull())
		assert.EqualStrings(t, "another-valid", elements[2].AsString())
	})
}

func TestTmSlugNullTokenize(t *testing.T) {
	rootdir := test.TempDir(t)
	ctx := eval.NewContext(stdlib.Functions(rootdir, []string{}))
	val, err := ctx.Eval(test.NewExpr(t, `{ a = "hello", b = tm_slug(null) }`))
	assert.NoError(t, err)

	tok := ast.TokensForValue(val)

	assert.EqualStrings(t, `{
a="hello"
b=null
}`, string(tok.Bytes()))
}
