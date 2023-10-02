// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build aix || android || darwin || dragonfly || freebsd || hurd || illumos || ios || linux || netbsd || openbsd || solaris

package test

import (
	"io/fs"
	"os"
)

// Chmod changes file permissions.
func Chmod(fname string, mode fs.FileMode) error {
	return os.Chmod(fname, mode)
}
