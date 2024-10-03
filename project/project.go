// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package project

import (
	"errors"
	"fmt"
	stdpath "path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/terramate-io/terramate/os"

	"github.com/zclconf/go-cty/cty"
)

// Path is a project path.
// The project paths can be either empty or an absolute forward slashed path with no lexical
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

var empty = Path{}

// NewPath creates a new project path.
// It panics if a relative path is provided.
func NewPath(p string) Path {
	if p == "" {
		return empty
	}
	if !stdpath.IsAbs(p) {
		panic(fmt.Errorf("project path must be absolute but got %q", p))
	}
	return newpath(p)
}

// assume the path is valid.
func newpath(p string) Path {
	return Path{
		path: stdpath.Clean(p),
	}
}

// Dir returns the path's directory.
func (p Path) Dir() Path {
	if p.path == "" {
		return p
	}
	return Path{
		path: stdpath.Dir(p.String()),
	}
}

// HostPath computes the absolute host path from the provided rootdir.
func (p Path) HostPath(rootdir os.Path) os.Path { return rootdir.Join(p.String()) }

// HasPrefix tests whether p begins with s prefix.
func (p Path) HasPrefix(s string) bool {
	return strings.HasPrefix(p.String(), s)
}

// HasDirPrefix tests whether p begins with a directory prefix s.
func (p Path) HasDirPrefix(s string) bool {
	if s == "/" {
		return strings.HasPrefix(p.String(), "/")
	}
	return s == p.String() || strings.HasPrefix(p.String(), s+"/")
}

// Join joins the pathstr path into p. See [stdpath.Join] for the underlying
// implementation.
func (p Path) Join(pathstr string) Path {
	return NewPath(stdpath.Join(p.String(), pathstr))
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
	if !stdpath.IsAbs(str) {
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
func PrjAbsPath(root, abspath os.Path) Path {
	d := filepath.ToSlash(abspath.TrimPrefix(root))
	if d == "" || d[0] != '/' {
		// handles: root=/ abspath=/
		//          root=/ abspath=/file
		d = "/" + d
	}
	return newpath(d)
}

// FriendlyFmtDir formats the directory in a friendly way for tooling output.
func FriendlyFmtDir(root, wd os.Path, dir string) (string, bool) {
	trimPart := PrjAbsPath(root, wd).String()
	if !strings.HasPrefix(dir, trimPart) {
		return "", false
	}

	dir = strings.TrimPrefix(dir, trimPart)

	if dir == "" {
		dir = "."
	} else if dir[0] == '/' {
		dir = dir[1:]
	}

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
