//go:build aix || android || darwin || dragonfly || freebsd || hurd || illumos || ios || linux || netbsd || openbsd || solaris

package test

import (
	"io/fs"
	"os"
)

func chmod(fname string, mode fs.FileMode) error {
	return os.Chmod(fname, mode)
}
