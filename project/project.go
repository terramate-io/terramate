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

// RelPath returns the dir relative to project's root.
func RelPath(root, dir string) string {
	log.Trace().
		Str("action", "RelPath()").
		Str("dir", dir).
		Str("root", root).
		Msg("Trim path to get relative dir.")

	d := strings.TrimPrefix(dir, root)

	if d == "" {
		d = "/"
	}

	return d
}

// AbsPath takes the root project dir and a dir path that is relative to the
// root project dir and returns an absolute path (relative to the host root).
func AbsPath(root, dir string) string {
	return filepath.Join(root, dir)
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

	trimPart := RelPath(root, wd)
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
