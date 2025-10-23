// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package tag provides helpers for dealing with Terramate tags.
package tag

import "github.com/terramate-io/terramate/errors"

// ErrInvalidTag indicates the tag is invalid.
const ErrInvalidTag errors.Kind = "invalid tag"

// Validate validates if the provided tag name is valid.
func Validate(tag string) error {
	for i, r := range tag {
		switch i {
		case 0:
			if !isLowerAlnum(r) {
				return errors.E(
					ErrInvalidTag,
					"%q: tags must start with lowercase alphanumeric ([0-9a-z])",
					tag)
			}
		case len(tag) - 1: // last rune
			if !isLowerAlnum(r) {
				return errors.E(
					ErrInvalidTag,
					"%q: tags must end with lowercase alphanumeric ([0-9a-z])",
					tag)
			}
		default:
			if !isLowerAlnum(r) && r != '-' && r != '_' && r != '.' && r != '/' {
				return errors.E(
					ErrInvalidTag,
					"%q: [a-z._-/] are the only permitted characters in tags",
					tag)
			}
		}
	}
	return nil
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func isLowerAlpha(r rune) bool {
	return (r >= 'a' && r <= 'z')
}
func isLowerAlnum(r rune) bool {
	return isLowerAlpha(r) || isDigit(r)
}
