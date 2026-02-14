// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package scaffold

import (
	"testing"

	"github.com/madlambda/spells/assert"
)

func TestFixupFileExtension(t *testing.T) {
	t.Parallel()

	type testcase struct {
		format string
		in     string
		want   string
	}

	for _, tc := range []testcase{{
		format: "yaml",
		in:     "blabla.tm",
		want:   "blabla.tm.yml",
	}, {
		format: "hcl",
		in:     "blabla.tm",
		want:   "blabla.tm.hcl",
	}, {
		format: "yaml",
		in:     "blabla.tm.hcl",
		want:   "blabla.tm.yml",
	}, {
		format: "hcl",
		in:     "blabla.tm.yaml",
		want:   "blabla.tm.hcl",
	}, {
		format: "hcl",
		in:     "blabla.tm.hcl",
		want:   "blabla.tm.hcl",
	}, {
		format: "yaml",
		in:     "blabla.tm.yml",
		want:   "blabla.tm.yml",
	}, {
		format: "yaml",
		in:     "blabla.txt",
		want:   "blabla.txt",
	}} {
		got := fixupFileExtension(tc.format, tc.in)
		assert.EqualStrings(t, tc.want, got)
	}
}
