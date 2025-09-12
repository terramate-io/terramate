// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stdlib

import (
	"github.com/terramate-io/terramate/errors"
	"github.com/zclconf/go-cty/cty"
)

// SlugWrongType is returned when tm_slug receives an invalid type.
const SlugWrongType errors.Kind = "tm_slug: wrong type"

// SlugListElementNotString is returned when a list element is not a string.
const SlugListElementNotString errors.Kind = "tm_slug: list element not string"

// SlugUnknownValue is returned when the value is not wholly known.
const SlugUnknownValue errors.Kind = "tm_slug: unknown value"

// errWrongRootType returns an error for invalid root types
func errWrongRootType(_ string, t cty.Type) error {
	return errors.E(SlugWrongType,
		"tm_slug: expected string or list(string), got %s", t.FriendlyName())
}

// errListElemNotString returns an error for non-string elements in lists
func errListElemNotString(_ string, idx int, t cty.Type) error {
	return errors.E(SlugListElementNotString,
		"tm_slug: list contains non-string element at index %d: %s", idx, t.FriendlyName())
}

// errUnknownValue returns an error for unknown/not wholly known values
func errUnknownValue(_ string) error {
	return errors.E(SlugUnknownValue, "tm_slug: value is not known")
}
