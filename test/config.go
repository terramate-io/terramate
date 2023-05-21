// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package test

import (
	"path/filepath"

	"github.com/terramate-io/terramate/config"
)

// FixupRangeOnAsserts fixes the range on all the given asserts.
// It assumes the asserts where created with relative paths and will
// join the relative path with the given dir to provide a final absolute path.
func FixupRangeOnAsserts(dir string, asserts []config.Assert) {
	for i := range asserts {
		asserts[i].Range.Filename = filepath.Join(dir, asserts[i].Range.Filename)
	}
}
