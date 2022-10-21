// Copyright 2022 Mineiros GmbH
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

package test

import (
	"path/filepath"

	"github.com/mineiros-io/terramate/config"
)

// FixupRangeOnAssert fixes the range on all the given asserts.
// It assumes the asserts where created with relative paths and will
// join the relative path with the given dir to provide a final absolute path.
// It won't change the given slice.
func FixupRangeOnAssert(dir string, asserts []config.Assert) {
	for i := range asserts {
		asserts[i].Range.Filename = filepath.Join(dir, asserts[i].Range.Filename)
	}
}
