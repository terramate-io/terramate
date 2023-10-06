// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build windows

package test

import (
	"io/fs"

	"github.com/hectane/go-acl"
)

// Chmod changes file permissions.
func Chmod(fname string, mode fs.FileMode) error {
	return acl.Chmod(fname, mode)
}
