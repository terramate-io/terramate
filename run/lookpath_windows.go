// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found at:
// https://github.com/golang/go/blob/616193510f45c6c588af9cb022dfdee52400d0ca/LICENSE

//go:build windows

package run

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/terramate-io/terramate/errors"
)

// ErrNotFound is the error resulting if a path search failed to find an executable file.
const ErrNotFound errors.Kind = "executable file not found in %PATH%"

// ErrDot indicates that a path lookup resolved to an executable
// in the current directory due to ‘.’ being in the path, either
// implicitly or explicitly. See the package documentation for details.
//
// Note that functions in this package do not return ErrDot directly.
// Code should use errors.Is(err, ErrDot), not err == ErrDot,
// to test whether a returned error err is due to this condition.
const ErrDot errors.Kind = "cannot run executable found relative to current directory"

func chkStat(file string) error {
	d, err := os.Stat(file)
	if err != nil {
		return err
	}
	if d.IsDir() {
		return fs.ErrPermission
	}
	return nil
}

func hasExt(file string) bool {
	i := strings.LastIndex(file, ".")
	if i < 0 {
		return false
	}
	return strings.LastIndexAny(file, `:\/`) < i
}

func findExecutable(file string, exts []string) (string, error) {
	if len(exts) == 0 {
		return file, chkStat(file)
	}
	if hasExt(file) {
		if chkStat(file) == nil {
			return file, nil
		}
	}
	for _, e := range exts {
		if f := file + e; chkStat(f) == nil {
			return f, nil
		}
	}
	return "", fs.ErrNotExist
}

// LookPath searches for an executable named file in the
// directories named by the PATH environment variable.
// LookPath also uses PATHEXT environment variable to match
// a suitable candidate.
// If file contains a slash, it is tried directly and the PATH is not consulted.
// Otherwise, on success, the result is an absolute path.
//
// In older versions of Go, LookPath could return a path relative to the current directory.
// As of Go 1.19, LookPath will instead return that path along with an error satisfying
// errors.Is(err, ErrDot). See the package documentation for more details.
func LookPath(file string, environ []string) (string, error) {
	var exts []string
	x, _ := getEnv(`PATHEXT`, environ)
	if x != "" {
		for _, e := range strings.Split(strings.ToLower(x), `;`) {
			if e == "" {
				continue
			}
			if e[0] != '.' {
				e = "." + e
			}
			exts = append(exts, e)
		}
	} else {
		exts = []string{".com", ".exe", ".bat", ".cmd"}
	}

	if strings.ContainsAny(file, `:\/`) {
		f, err := findExecutable(file, exts)
		if err == nil {
			return f, nil
		}
		return "", errors.E(ErrNotFound, err, file)
	}

	// On Windows, creating the NoDefaultCurrentDirectoryInExePath
	// environment variable (with any value or no value!) signals that
	// path lookups should skip the current directory.
	// In theory we are supposed to call NeedCurrentDirectoryForExePathW
	// "as the registry location of this environment variable can change"
	// but that seems exceedingly unlikely: it would break all users who
	// have configured their environment this way!
	// https://docs.microsoft.com/en-us/windows/win32/api/processenv/nf-processenv-needcurrentdirectoryforexepathw
	// See also go.dev/issue/43947.
	var (
		dotf   string
		dotErr error
	)
	if _, found := getEnv("NoDefaultCurrentDirectoryInExePath", environ); !found {
		if f, err := findExecutable(filepath.Join(".", file), exts); err == nil {
			dotf, dotErr = f, errors.E(ErrDot, file)
		}
	}

	path, _ := getEnv("path", environ)
	for _, dir := range filepath.SplitList(path) {
		if f, err := findExecutable(filepath.Join(dir, file), exts); err == nil {
			if dotErr != nil {
				// https://go.dev/issue/53536: if we resolved a relative path implicitly,
				// and it is the same executable that would be resolved from the explicit %PATH%,
				// prefer the explicit name for the executable (and, likely, no error) instead
				// of the equivalent implicit name with ErrDot.
				//
				// Otherwise, return the ErrDot for the implicit path as soon as we find
				// out that the explicit one doesn't match.
				dotfi, dotfiErr := os.Lstat(dotf)
				fi, fiErr := os.Lstat(f)
				if dotfiErr != nil || fiErr != nil || !os.SameFile(dotfi, fi) {
					return dotf, dotErr
				}
			}

			return f, nil
		}
	}

	if dotErr != nil {
		return dotf, dotErr
	}
	return "", errors.E(ErrNotFound, file)
}
