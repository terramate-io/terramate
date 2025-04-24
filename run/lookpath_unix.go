// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found at:
// https://github.com/golang/go/blob/616193510f45c6c588af9cb022dfdee52400d0ca/LICENSE

//go:build aix || android || darwin || dragonfly || freebsd || hurd || illumos || ios || linux || netbsd || openbsd || solaris

package run

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/terramate-io/terramate/errors"
)

// ErrNotFound is the error resulting if a path search failed to find an executable file.
const ErrNotFound errors.Kind = "executable file not found in $PATH"

// LookPath searches for an executable named file in the
// directories named by the PATH environment variable.
// If file contains a slash, it is tried directly and the PATH is not consulted.
// Otherwise, on success, the result is an absolute path.
//
// In older versions of Go, LookPath could return a path relative to the current directory.
// As of Go 1.19, LookPath will instead return that path along with an error satisfying
// errors.Is(err, ErrDot). See the package documentation for more details.
func LookPath(file string, cwd string, environ []string) (string, error) {
	// NOTE(rsc): I wish we could use the Plan 9 behavior here
	// (only bypass the path if file begins with / or ./ or ../)
	// but that would not match all the Unix shells.

	if strings.Contains(file, "/") {
		if !filepath.IsAbs(file) {
			file = filepath.Join(cwd, file)
		}
		err := findExecutable(file)
		if err == nil {
			return file, nil
		}
		return "", errors.E(ErrNotFound, err, file)
	}
	path, _ := Getenv("PATH", environ)
	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			// Unix shell semantics: path element "" means "."
			dir = "."
		}
		path := filepath.Join(dir, file)
		if err := findExecutable(path); err == nil {
			return path, nil
		}
	}
	return "", errors.E(ErrNotFound, file)
}

func findExecutable(file string) error {
	d, err := os.Stat(file)
	if err != nil {
		return err
	}
	m := d.Mode()
	if m.IsDir() {
		return syscall.EISDIR
	}

	err = unix.Access(file, unix.X_OK)
	// ENOSYS means Eaccess is not available or not implemented.
	// EPERM can be returned by Linux containers employing seccomp.
	// In both cases, fall back to checking the permission bits.
	if err == nil || (err != syscall.ENOSYS && err != syscall.EPERM) {
		return err
	}
	if m&0111 != 0 {
		return nil
	}
	return fs.ErrPermission
}
