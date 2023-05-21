// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build js

package test

import (
	"io/fs"
)

func chmod(fname string, mode fs.FileMode) error {
	panic("`chmod` is not implemented for wasm")
}
