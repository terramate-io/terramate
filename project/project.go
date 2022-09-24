// Copyright 2021 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package project

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/mineiros-io/terramate/errors"
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
type Path string

// Paths is a list of project paths.
type Paths []Path

func NewPath(p string) Path {
	if !path.IsAbs(p) {
		panic(errors.E("project path must be absolute but got %s", p))
	}
	return Path(path.Clean(p))
}

// Dir returns the path's directory.
func (p Path) Dir() Path { return Path(path.Dir(p.String())) }

// HasPrefix tests whether p begins with s prefix.
func (p Path) HasPrefix(s string) bool {
	return strings.HasPrefix(p.String(), s)
}

// String returns the path as a string.
func (p Path) String() string { return string(p) }

func (paths Paths) Strings() []string {
	vals := []string{}
	for _, p := range paths {
		vals = append(vals, p.String())
	}
	return vals
}

// Metadata represents project wide metadata.
type Metadata struct {
	rootdir string
	stacks  Paths
}

// NewMetadata creates a new project metadata.
func NewMetadata(rootdir string, stackpaths Paths) Metadata {
	if !filepath.IsAbs(rootdir) {
		panic("rootdir must be an absolute path")
	}
	return Metadata{
		rootdir: rootdir,
		stacks:  stackpaths,
	}
}

// Rootdir is the root dir of the project
func (m Metadata) Rootdir() string {
	return m.rootdir
}

// Stacks contains the absolute path relative to the project root
// of all stacks inside the project.
func (m Metadata) Stacks() Paths { return m.stacks }

// ToCtyMap returns the project metadata as a cty.Value map.
func (m Metadata) ToCtyMap() map[string]cty.Value {
	rootfs := cty.ObjectVal(map[string]cty.Value{
		"absolute": cty.StringVal(m.Rootdir()),
		"basename": cty.StringVal(filepath.Base(m.Rootdir())),
	})
	rootpath := cty.ObjectVal(map[string]cty.Value{
		"fs": rootfs,
	})
	root := cty.ObjectVal(map[string]cty.Value{
		"path": rootpath,
	})
	stacksNs := cty.ObjectVal(map[string]cty.Value{
		"list": toCtyStringList(m.Stacks().Strings()),
	})
	return map[string]cty.Value{
		"root":   root,
		"stacks": stacksNs,
	}
}

// PrjAbsPath converts the file system absolute path absdir into an absolute
// project path on the form /path/on/project relative to the given root.
func PrjAbsPath(root, abspath string) Path {
	log.Trace().
		Str("action", "PrjAbsPath()").
		Str("path", abspath).
		Str("root", root).
		Msg("Trim path to get relative dir.")

	d := filepath.ToSlash(strings.TrimPrefix(abspath, root))
	if d == "" {
		d = "/"
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

func toCtyStringList(list []string) cty.Value {
	if len(list) == 0 {
		// cty panics if the list is empty
		return cty.ListValEmpty(cty.String)
	}
	res := make([]cty.Value, len(list))
	for i, elem := range list {
		res[i] = cty.StringVal(elem)
	}
	return cty.ListVal(res)
}
