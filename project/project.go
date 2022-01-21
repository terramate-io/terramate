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
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
)

// PrjAbsPath converts the file system absolute path absdir into an absolute
// project path on the form /path/on/project relative to the given root.
func PrjAbsPath(root, absdir string) string {
	log.Trace().
		Str("action", "PrjAbsPath()").
		Str("dir", absdir).
		Str("root", root).
		Msg("Trim path to get relative dir.")

	d := strings.TrimPrefix(absdir, root)
	if d == "" {
		d = "/"
	}

	return d
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

	trimPart := PrjAbsPath(root, wd)
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
