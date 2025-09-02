// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tag_test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/config/tag"
)

func TestTagValidate(t *testing.T) {
	type testcase struct {
		tag     string
		isValid bool
	}

	for _, tc := range []testcase{{
		tag:     "",
		isValid: true,
	}, {
		tag:     "some_tag",
		isValid: true,
	}, {
		tag:     "some_tag/with-a/slash",
		isValid: true,
	}, {
		tag:     "some_tag/with?question",
		isValid: false,
	}, {
		tag:     "trailing_",
		isValid: false,
	}, {
		tag:     "trailing_slash/",
		isValid: false,
	}, {
		tag:     "no whitespace",
		isValid: false,
	}, {
		tag:     "nö",
		isValid: false,
	}} {
		t.Run(tc.tag, func(t *testing.T) {
			t.Parallel()
			err := tag.Validate(tc.tag)
			if tc.isValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
