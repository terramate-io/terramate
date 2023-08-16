// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package project

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
)

// Path is a project path.
// The project paths are always absolute forward slashed paths with no lexical
// processing left, which means they must be cleaned paths. See:
//
//	https://pkg.go.dev/path#Clean
//
// The project path has / as root.
type Path struct {
	path string
}

// Paths is a list of project paths.
type Paths []Path

// Runtime is a map of runtime values exposed in the terramate namespace.
type Runtime map[string]cty.Value

// MaxGlobalLabels allowed to be used in a globals block.
// TODO(i4k): get rid of this limit.
const MaxGlobalLabels = 256

// NewPath creates a new project path.
// It panics if a relative path is provided.
func NewPath(p string) Path {
	if !path.IsAbs(p) {
		panic(fmt.Errorf("project path must be absolute but got %q", p))
	}
	return Path{
		path: path.Clean(p),
	}
}

// Dir returns the path's directory.
func (p Path) Dir() Path {
	return Path{
		path: path.Dir(p.String()),
	}
}

// HostPath computes the absolute host path from the provided rootdir.
func (p Path) HostPath(rootdir string) string {
	return filepath.Join(rootdir, filepath.FromSlash(p.path))
}

// HasPrefix tests whether p begins with s prefix.
func (p Path) HasPrefix(s string) bool {
	return strings.HasPrefix(p.String(), s)
}

// Join joins the pathstr path into p. See [path.Join] for the underlying
// implementation.
func (p Path) Join(pathstr string) Path {
	return NewPath(path.Join(p.String(), pathstr))
}

// String returns the path as a string.
func (p Path) String() string { return p.path }

// MarshalJSON implements the json.Marshaler interface
func (p Path) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(p.String())), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (p *Path) UnmarshalJSON(data []byte) error {
	str, err := strconv.Unquote(string(data))
	if err != nil {
		return err
	}
	if !path.IsAbs(str) {
		return errors.New(`a project path must start with "/"`)
	}
	p2 := NewPath(str)
	*p = p2
	return nil
}

// Strings returns a []string from the []Path.
func (paths Paths) Strings() []string {
	vals := []string{}
	for _, p := range paths {
		vals = append(vals, p.String())
	}
	return vals
}

// Sort paths in-place.
func (paths Paths) Sort() {
	sort.Slice(paths, func(i, j int) bool {
		return string(paths[i].path) < string(paths[j].path)
	})
}

// PrjAbsPath converts the file system absolute path absdir into an absolute
// project path on the form /path/on/project relative to the given root.
func PrjAbsPath(root, abspath string) Path {
	d := filepath.ToSlash(strings.TrimPrefix(abspath, root))
	if d == "" {
		d = "/"
	}
	if d[0] != '/' {
		// handle root=/ abspath=/file
		d = "/" + d
	}
	return NewPath(d)
}

// AbsPath takes the root project dir and a project's absolute path prjAbsPath
// and returns an absolute path to the file system.
func AbsPath(root, prjAbsPath string) string {
	return filepath.Join(root, prjAbsPath)
}

// FriendlyFmtDir formats the directory in a friendly way for tooling output.
func FriendlyFmtDir(root, wd, dir string) (string, bool) {
	logger := log.With().
		Str("action", "FriendlyFmtDir()").
		Logger()

	logger.Trace().
		Str("prefix", wd).
		Str("root", root).
		Str("dir", dir).
		Msg("Get relative path.")

	trimPart := PrjAbsPath(root, wd).String()
	if !strings.HasPrefix(dir, trimPart) {
		return "", false
	}

	logger.Trace().
		Str("dir", dir).
		Str("prefix", trimPart).
		Msg("Trim prefix.")
	dir = strings.TrimPrefix(dir, trimPart)

	if dir == "" {
		dir = "."
	} else if dir[0] == '/' {
		dir = dir[1:]
	}

	logger.Trace().
		Str("newdir", dir).
		Msg("Get friendly dir.")

	return dir, true
}

// Merge other runtime values into the current set.
func (runtime Runtime) Merge(other Runtime) {
	for k, v := range other {
		if _, ok := runtime[k]; ok {
			panic(fmt.Errorf("runtime key %s conflicts", k))
		}
		runtime[k] = v
	}
}
