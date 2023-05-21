// Copyright 2023 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
			if !isLowerAlpha(r) {
				return errors.E(
					ErrInvalidTag,
					"%q: tags must start with lowercase alphabetic character ([a-z])",
					tag)
			}
		case len(tag) - 1: // last rune
			if !isLowerAlnum(r) {
				return errors.E(
					ErrInvalidTag,
					"%q: tags must end with lowercase alphanumeric ([0-9a-z]+)",
					tag)
			}
		default:
			if !isLowerAlnum(r) && r != '-' && r != '_' {
				return errors.E(
					ErrInvalidTag,
					"%q: [a-z_-] are the only permitted characters in tags",
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
