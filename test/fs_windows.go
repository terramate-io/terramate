//go:build windows

package test

import (
	"io/fs"

	"github.com/hectane/go-acl"
)

func chmod(fname string, mode fs.FileMode) error {
	return acl.Chmod(fname, mode)
}
