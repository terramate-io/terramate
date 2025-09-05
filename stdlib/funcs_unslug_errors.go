// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stdlib

import (
	"fmt"
	"strings"

	"github.com/terramate-io/terramate/errors"
)

// Error kinds for tm_unslug function
const (
	// ErrUnslugIndeterministic indicates that a slug maps to multiple dictionary entries
	ErrUnslugIndeterministic errors.Kind = "tm_unslug indeterministic mapping"

	// ErrUnslugInvalidDictionaryType indicates the dictionary contains non-string elements
	ErrUnslugInvalidDictionaryType errors.Kind = "tm_unslug invalid dictionary type"

	// ErrUnslugInvalidValueType indicates the value is not a string or list of strings
	ErrUnslugInvalidValueType errors.Kind = "tm_unslug invalid value type"

	// ErrUnslugInternal indicates an unexpected internal failure
	ErrUnslugInternal errors.Kind = "tm_unslug internal error"
)

// NewUnslugErrorIndeterministic creates an error for when a slug maps to multiple dictionary entries
func NewUnslugErrorIndeterministic(slug string, candidates []string) error {
	quotedCandidates := make([]string, len(candidates))
	for i, c := range candidates {
		quotedCandidates[i] = fmt.Sprintf("%q", c)
	}
	msg := fmt.Sprintf("slug %q maps to multiple dictionary entries: [%s]",
		slug, strings.Join(quotedCandidates, ", "))
	return errors.E(ErrUnslugIndeterministic, msg)
}
