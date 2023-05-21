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
