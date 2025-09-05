// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stdlib_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/stdlib"
	"github.com/terramate-io/terramate/test"
	"github.com/zclconf/go-cty/cty"
)

func TestUnslugFunction(t *testing.T) {
	type testcase struct {
		expr string
		want interface{} // string for success, []string for list success, errors.Kind for errors
	}

	// Happy path tests
	t.Run("single string round-trip", func(t *testing.T) {
		testcases := []testcase{
			{
				expr: `tm_unslug(tm_slug("Team Alpha"), ["Team Alpha", "Team Beta"])`,
				want: "Team Alpha",
			},
			{
				expr: `tm_unslug("team-alpha", ["Team Alpha", "Team Beta"])`,
				want: "Team Alpha",
			},
			{
				expr: `tm_unslug("team-beta", ["Team Alpha", "Team Beta"])`,
				want: "Team Beta",
			},
		}

		for _, tc := range testcases {
			t.Run(tc.expr, func(t *testing.T) {
				runUnslugTest(t, tc)
			})
		}
	})

	t.Run("list round-trip", func(t *testing.T) {
		testcases := []testcase{
			{
				expr: `tm_unslug(["team-alpha", "team-beta"], ["Team Alpha", "Team Beta"])`,
				want: []string{"Team Alpha", "Team Beta"},
			},
		}

		for _, tc := range testcases {
			t.Run(tc.expr, func(t *testing.T) {
				runUnslugTest(t, tc)
			})
		}
	})

	t.Run("no match fallback", func(t *testing.T) {
		testcases := []testcase{
			{
				expr: `tm_unslug("team-gamma", ["Team Alpha", "Team Beta"])`,
				want: "team-gamma",
			},
			{
				expr: `tm_unslug(["team-alpha", "team-gamma"], ["Team Alpha", "Team Beta"])`,
				want: []string{"Team Alpha", "team-gamma"},
			},
		}

		for _, tc := range testcases {
			t.Run(tc.expr, func(t *testing.T) {
				runUnslugTest(t, tc)
			})
		}
	})

	t.Run("special characters and normalization", func(t *testing.T) {
		testcases := []testcase{
			{
				// Trailing separator tolerated - using a case where original doesn't have trailing separator
				expr: `tm_unslug("hello-world--", ["Hello World"])`, // extra trailing dashes
				want: "Hello World",
			},
			{
				// Repeated internal separators tolerated
				expr: `tm_unslug("hello--world", ["Hello World"])`,
				want: "Hello World",
			},
			{
				// Leading separator NOT tolerated (no match)
				expr: `tm_unslug("-hello-world", ["Hello World"])`,
				want: "-hello-world",
			},
		}

		for _, tc := range testcases {
			t.Run(tc.expr, func(t *testing.T) {
				runUnslugTest(t, tc)
			})
		}
	})

	t.Run("collisions", func(t *testing.T) {
		testcases := []testcase{
			{
				// Multiple candidates for same slug should error
				expr: `tm_unslug("team-alpha", ["Team Alpha", "Team-Alpha"])`,
				want: stdlib.ErrUnslugIndeterministic,
			},
		}

		for _, tc := range testcases {
			t.Run(tc.expr, func(t *testing.T) {
				runUnslugTest(t, tc)
			})
		}
	})

	t.Run("type validation", func(t *testing.T) {
		testcases := []testcase{
			{
				// Dictionary contains non-string
				expr: `tm_unslug("x", ["A", 1])`,
				want: stdlib.ErrUnslugInvalidDictionaryType,
			},
			{
				// Value list contains non-string
				expr: `tm_unslug(["a", 2], ["A", "B"])`,
				want: stdlib.ErrUnslugInvalidValueType,
			},
			{
				// Invalid value type
				expr: `tm_unslug(123, ["A", "B"])`,
				want: stdlib.ErrUnslugInvalidValueType,
			},
		}

		for _, tc := range testcases {
			t.Run(tc.expr, func(t *testing.T) {
				runUnslugTest(t, tc)
			})
		}
	})

	t.Run("edge cases", func(t *testing.T) {
		testcases := []testcase{
			{
				// Empty dictionary
				expr: `tm_unslug("test", [])`,
				want: "test",
			},
			{
				// Empty string
				expr: `tm_unslug("", ["test"])`,
				want: "",
			},
			{
				// Empty list
				expr: `tm_unslug([], ["test"])`,
				want: []string{},
			},
		}

		for _, tc := range testcases {
			t.Run(tc.expr, func(t *testing.T) {
				runUnslugTest(t, tc)
			})
		}
	})
}

// runUnslugTest runs a single test case for tm_unslug function
func runUnslugTest(t *testing.T, tc struct {
	expr string
	want interface{} // string for success, []string for list success, errors.Kind for errors
}) {
	t.Helper()

	rootdir := test.TempDir(t)
	ctx := eval.NewContext(stdlib.Functions(rootdir, []string{}))

	result, err := ctx.Eval(test.NewExpr(t, tc.expr))

	// Check if we expect an error
	switch expected := tc.want.(type) {
	case errors.Kind:
		if err == nil {
			t.Fatalf("expected error with kind %v but got success with value: %v", expected, result.GoString())
		}
		// Check if the error contains the expected kind (it may be wrapped in HCL function call error)
		if !errors.IsKind(err, expected) && !strings.Contains(err.Error(), string(expected)) {
			t.Fatalf("expected error kind %v but got error: %v", expected, err)
		}

	case string:
		// Expect successful string result
		if err != nil {
			t.Fatalf("expected success but got error: %v", err)
		}
		if result.Type() != cty.String {
			t.Fatalf("expected string result but got type: %s", result.Type().GoString())
		}
		got := result.AsString()
		assert.EqualStrings(t, expected, got)

	case []string:
		// Expect successful list result
		if err != nil {
			t.Fatalf("expected success but got error: %v", err)
		}
		if !result.Type().IsListType() && !result.Type().IsTupleType() {
			t.Fatalf("expected list/tuple result but got type: %s", result.Type().GoString())
		}

		var got []string
		for it := result.ElementIterator(); it.Next(); {
			_, elem := it.Element()
			if elem.Type() != cty.String {
				t.Fatalf("expected string elements but got: %s", elem.Type().GoString())
			}
			got = append(got, elem.AsString())
		}

		if len(got) != len(expected) {
			t.Fatalf("expected %d elements but got %d: %v", len(expected), len(got), got)
		}

		for i, expectedElem := range expected {
			assert.EqualStrings(t, expectedElem, got[i])
		}

	default:
		t.Fatalf("unsupported test expectation type: %T", tc.want)
	}
}

// TestUnslugNormalization tests the normalization rules more thoroughly
func TestUnslugNormalization(t *testing.T) {
	testCases := []struct {
		name  string
		input string
		want  string
	}{
		{"no change needed", "hello-world", "Hello World"},
		{"trailing separator", "hello-world--", "Hello World"},            // extra trailing dash should be stripped
		{"multiple trailing separators", "hello-world---", "Hello World"}, // multiple trailing dashes should be stripped
		{"repeated internal separators", "hello--world", "Hello World"},
		{"mixed repeated separators", "hello---world--", "Hello World"},
		{"leading separator preserved", "-hello-world", "-hello-world"}, // no match, returns input
	}

	dictionary := `["Hello World"]`

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			expr := fmt.Sprintf(`tm_unslug("%s", %s)`, tc.input, dictionary)

			rootdir := test.TempDir(t)
			ctx := eval.NewContext(stdlib.Functions(rootdir, []string{}))

			result, err := ctx.Eval(test.NewExpr(t, expr))
			assert.NoError(t, err)

			got := result.AsString()
			assert.EqualStrings(t, tc.want, got)
		})
	}
}

// TestUnslugInvariant tests the documented invariant that for all words in dictionary:
// tm_unslug(tm_slug(word), dictionary) == word
// Note: This only holds for words that don't lose information during slugification
func TestUnslugInvariant(t *testing.T) {
	testWords := []string{
		"Team Alpha",
		"Team Beta",
		"UPPERCASE",
		"lowercase",
		"Mixed-Case_With123Numbers",
		"", // edge case
	}

	for _, word := range testWords {
		t.Run(fmt.Sprintf("word_%s", strings.ReplaceAll(word, " ", "_")), func(t *testing.T) {
			// Create dictionary with this word and some others to avoid trivial cases
			dictionary := fmt.Sprintf(`["%s", "Other Word", "Another Entry"]`,
				strings.ReplaceAll(word, `"`, `\"`))

			expr := fmt.Sprintf(`tm_unslug(tm_slug("%s"), %s)`,
				strings.ReplaceAll(word, `"`, `\"`), dictionary)

			rootdir := test.TempDir(t)
			ctx := eval.NewContext(stdlib.Functions(rootdir, []string{}))

			result, err := ctx.Eval(test.NewExpr(t, expr))
			assert.NoError(t, err)

			got := result.AsString()
			assert.EqualStrings(t, word, got)
		})
	}
}

// TestUnslugInformationLoss tests cases where information is lost during slugification
// but the round-trip still works because the original is in the dictionary
func TestUnslugInformationLoss(t *testing.T) {
	testCases := []struct {
		name     string
		word     string
		expected string // what we expect to get back (the original)
	}{
		{"special characters lost", "Special Characters!@#", "Special Characters!@#"},
		{"multiple spaces normalized", "Multiple   Spaces", "Multiple   Spaces"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create dictionary with the original word
			dictionary := fmt.Sprintf(`["%s", "Other Word"]`,
				strings.ReplaceAll(tc.word, `"`, `\"`))

			expr := fmt.Sprintf(`tm_unslug(tm_slug("%s"), %s)`,
				strings.ReplaceAll(tc.word, `"`, `\"`), dictionary)

			rootdir := test.TempDir(t)
			ctx := eval.NewContext(stdlib.Functions(rootdir, []string{}))

			result, err := ctx.Eval(test.NewExpr(t, expr))
			assert.NoError(t, err)

			got := result.AsString()
			// Should get back the original word since it's in the dictionary
			assert.EqualStrings(t, tc.expected, got)
		})
	}
}

// TestUnslugSpecialCharacters tests extensive special character handling
func TestUnslugSpecialCharacters(t *testing.T) {
	testCases := []struct {
		name       string
		value      string
		dictionary []string
		expected   string
	}{
		{
			name:       "punctuation_marks",
			value:      "hello-world",
			dictionary: []string{"Hello, World!", "Test"},
			expected:   "Hello, World!",
		},
		{
			name:       "brackets_and_braces",
			value:      "test-item",
			dictionary: []string{"Test [Item]", "Another"},
			expected:   "Test [Item]",
		},
		{
			name:       "mathematical_symbols",
			value:      "a-b-c-d",
			dictionary: []string{"a+b=c/d", "Test"},
			expected:   "a+b=c/d",
		},
		{
			name:       "currency_symbols",
			value:      "-100",
			dictionary: []string{"$100", "Other"}, // Only one currency symbol to avoid collision
			expected:   "$100",
		},
		{
			name:       "mixed_unicode",
			value:      "caf", // Café slugs to "caf-" which normalizes to "caf"
			dictionary: []string{"Café", "Cafe"},
			expected:   "Café",
		},
		{
			name:       "emoji_support",
			value:      "hello-world",
			dictionary: []string{"Hello 👋 World", "Test"},
			expected:   "Hello 👋 World",
		},
		{
			name:       "html_entities",
			value:      "copyright-2024",
			dictionary: []string{"Copyright © 2024", "Test"},
			expected:   "Copyright © 2024",
		},
		{
			name:       "quotes_and_apostrophes",
			value:      "john-s-code",
			dictionary: []string{"John's \"Code\"", "Test"},
			expected:   "John's \"Code\"",
		},
		{
			name:       "tabs_and_newlines",
			value:      "line1-line2",
			dictionary: []string{"Line1\tLine2", "Other"}, // Tab character
			expected:   "Line1\tLine2",
		},
		{
			name:       "special_separators",
			value:      "path-to-file",
			dictionary: []string{"path/to/file", "Other"}, // Forward slash
			expected:   "path/to/file",
		},
		{
			name:       "ampersands_and_pipes",
			value:      "a-b-c",
			dictionary: []string{"A & B | C", "Test"},
			expected:   "A & B | C",
		},
		{
			name:       "xml_html_tags",
			value:      "-b-text-b", // <b>text</b> slugs to "-b-text--b-" which normalizes to "-b-text-b"
			dictionary: []string{"<b>text</b>", "Test"},
			expected:   "<b>text</b>",
		},
		{
			name:       "percent_encoding",
			value:      "hello-20world",
			dictionary: []string{"Hello%20World", "Test"},
			expected:   "Hello%20World",
		},
		{
			name:       "underscores_preserved_in_dictionary",
			value:      "snake_case",
			dictionary: []string{"snake_case", "snake-case"},
			expected:   "snake_case",
		},
		{
			name:       "mixed_case_and_special",
			value:      "the-quick-brown-fox",
			dictionary: []string{"The QUICK (Brown) Fox!", "Test"},
			expected:   "The QUICK (Brown) Fox!",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fn := stdlib.UnslugFunc()

			dictValues := make([]cty.Value, len(tc.dictionary))
			for i, v := range tc.dictionary {
				dictValues[i] = cty.StringVal(v)
			}

			result, err := fn.Call([]cty.Value{
				cty.StringVal(tc.value),
				cty.ListVal(dictValues),
			})

			assert.NoError(t, err)
			assert.EqualStrings(t, tc.expected, result.AsString())
		})
	}
}

// TestUnslugSpecialCharactersList tests special character handling with lists
func TestUnslugSpecialCharactersList(t *testing.T) {
	fn := stdlib.UnslugFunc()

	// Test with a list of values containing special characters
	values := cty.ListVal([]cty.Value{
		cty.StringVal("hello-world"),
		cty.StringVal("foo-bar"),
		cty.StringVal("test-123"),
	})

	dictionary := cty.ListVal([]cty.Value{
		cty.StringVal("Hello, World!"),
		cty.StringVal("Foo & Bar"),
		cty.StringVal("Test #123"),
		cty.StringVal("Other"),
	})

	result, err := fn.Call([]cty.Value{values, dictionary})
	assert.NoError(t, err)

	expectedList := cty.ListVal([]cty.Value{
		cty.StringVal("Hello, World!"),
		cty.StringVal("Foo & Bar"),
		cty.StringVal("Test #123"),
	})

	if !result.RawEquals(expectedList) {
		t.Fatalf("expected %v, got %v", expectedList, result)
	}
}
