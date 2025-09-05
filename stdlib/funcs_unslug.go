// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stdlib

import (
	"regexp"
	"strings"

	"github.com/terramate-io/terramate/errors"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

// UnslugFunc returns the tm_unslug function.
func UnslugFunc() function.Function {
	return function.New(&function.Spec{
		Description: `Maps a slug (or list of slugs) back to the original human-readable string(s) using a dictionary.`,
		Params: []function.Parameter{
			{
				Name:        "value",
				Type:        cty.DynamicPseudoType,
				Description: "The slug or list of slugs to unslug",
			},
			{
				Name:        "dictionary",
				Type:        cty.DynamicPseudoType,
				Description: "List of unslugged canonical values",
			},
		},
		Type: unslugType,
		Impl: unslugImpl,
	})
}

// unslugType determines the return type based on the input value type
func unslugType(args []cty.Value) (cty.Type, error) {
	value := args[0]
	dictionary := args[1]

	// Validate dictionary is a list of strings
	if !dictionary.Type().IsListType() && !dictionary.Type().IsTupleType() {
		return cty.NilType, errors.E(ErrUnslugInvalidDictionaryType, "dictionary must be a list of strings")
	}

	// If dictionary is known, validate all elements are strings
	if dictionary.IsKnown() && !dictionary.IsNull() {
		for it := dictionary.ElementIterator(); it.Next(); {
			_, elem := it.Element()
			if elem.Type() != cty.String {
				return cty.NilType, errors.E(ErrUnslugInvalidDictionaryType, "dictionary must contain only strings")
			}
		}
	}

	// Validate value type and return matching type
	valueType := value.Type()

	if valueType == cty.String {
		return cty.String, nil
	}

	if valueType.IsListType() || valueType.IsTupleType() {
		// Validate list elements are strings if known
		if value.IsKnown() && !value.IsNull() {
			for it := value.ElementIterator(); it.Next(); {
				_, elem := it.Element()
				if elem.Type() != cty.String {
					return cty.NilType, errors.E(ErrUnslugInvalidValueType, "value list must contain only strings")
				}
			}
		}
		return cty.List(cty.String), nil
	}

	// Default case - not a valid type

	return cty.NilType, errors.E(ErrUnslugInvalidValueType, "value must be a string or list of strings")
}

// unslugImpl implements the tm_unslug function logic
func unslugImpl(args []cty.Value, retType cty.Type) (cty.Value, error) {
	value := args[0]
	dictionary := args[1]

	// Handle null/unknown values
	if value.IsNull() {
		return cty.NullVal(retType), nil
	}
	if !value.IsKnown() || !dictionary.IsKnown() {
		return cty.UnknownVal(retType), nil
	}

	// Build the lookup map from dictionary
	lookupMap, err := buildLookupMap(dictionary)
	if err != nil {
		return cty.NilVal, err
	}

	// Process based on value type
	if value.Type() == cty.String {
		result, err := unslugString(value.AsString(), lookupMap)
		if err != nil {
			return cty.NilVal, err
		}
		return cty.StringVal(result), nil
	}

	// Process list of strings
	var results []cty.Value
	for it := value.ElementIterator(); it.Next(); {
		_, elem := it.Element()
		if elem.Type() != cty.String {
			return cty.NilVal, errors.E(ErrUnslugInvalidValueType, "list contains non-string element")
		}
		if !elem.IsKnown() {
			results = append(results, cty.UnknownVal(cty.String))
			continue
		}
		if elem.IsNull() {
			results = append(results, cty.NullVal(cty.String))
			continue
		}

		result, err := unslugString(elem.AsString(), lookupMap)
		if err != nil {
			return cty.NilVal, err
		}
		results = append(results, cty.StringVal(result))
	}

	if len(results) == 0 {
		return cty.ListValEmpty(cty.String), nil
	}
	return cty.ListVal(results), nil
}

// buildLookupMap creates a map from normalized slugs to original dictionary entries
func buildLookupMap(dictionary cty.Value) (map[string][]string, error) {
	lookupMap := make(map[string][]string)

	for it := dictionary.ElementIterator(); it.Next(); {
		_, elem := it.Element()
		if elem.Type() != cty.String {
			return nil, errors.E(ErrUnslugInvalidDictionaryType, "dictionary contains non-string element")
		}
		if !elem.IsKnown() {
			continue // Skip unknown values in dictionary
		}
		if elem.IsNull() {
			continue // Skip null values in dictionary
		}

		original := elem.AsString()
		slugged := slugify(original)
		normalized := normalizeSlug(slugged)

		lookupMap[normalized] = append(lookupMap[normalized], original)
	}

	return lookupMap, nil
}

// unslugString maps a single slug back to its original form using the lookup map
func unslugString(inputSlug string, lookupMap map[string][]string) (string, error) {
	normalized := normalizeSlug(inputSlug)

	candidates, found := lookupMap[normalized]
	if !found || len(candidates) == 0 {
		// No match found, return the input slug unchanged
		return inputSlug, nil
	}

	if len(candidates) > 1 {
		// Multiple matches found, raise an error
		return "", NewUnslugErrorIndeterministic(inputSlug, candidates)
	}

	// Exactly one match found
	return candidates[0], nil
}

// normalizeSlug normalizes a slug for matching:
// - Lowercase
// - Collapse repeated internal separators
// - Strip trailing separators only (not leading)
func normalizeSlug(s string) string {
	s = strings.ToLower(s)

	// Collapse repeated internal separators
	repeatedSep := regexp.MustCompile(`-{2,}`)
	s = repeatedSep.ReplaceAllString(s, "-")

	// Strip trailing separators only
	s = strings.TrimRight(s, "-")

	return s
}
