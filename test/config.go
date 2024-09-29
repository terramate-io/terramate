// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package test

import (
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/os"
)

// FixupRangeOnAsserts fixes the range on all the given asserts.
// It assumes the asserts where created with relative paths and will
// join the relative path with the given dir to provide a final absolute path.
func FixupRangeOnAsserts(dir os.Path, asserts []config.Assert) {
	for i := range asserts {
		asserts[i].Range.Filename = dir.Join(asserts[i].Range.Filename).String()
	}
}
