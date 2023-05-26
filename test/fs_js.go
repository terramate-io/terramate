// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build js

package test

import (
	"io/fs"
)

func chmod(fname string, mode fs.FileMode) error {
	panic("`chmod` is not implemented for wasm")
}
