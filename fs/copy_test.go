// Copyright 2022 Mineiros GmbH
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

package fs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/fs"
	"github.com/mineiros-io/terramate/test/sandbox"
	"github.com/rs/zerolog"
)

func TestCopyIfAllFilesAreFilteredDirIsNotCreated(t *testing.T) {
	s := sandbox.New(t)
	s.BuildTree([]string{
		"f:test/1",
		"f:test/2",
		"f:test/3",
		"f:test/sub/notcopy",
		"f:test3/notcopy",
	})

	destdir := t.TempDir()
	err := fs.CopyDir(destdir, s.RootDir(), func(path string, entry os.DirEntry) bool {
		return entry.Name() != "notcopy" &&
			entry.Name() != ".git" &&
			entry.Name() != "README.md"
	})

	assert.NoError(t, err)

	entries, err := os.ReadDir(destdir)
	assert.NoError(t, err)
	assert.EqualInts(t, 1, len(entries))
	assert.EqualStrings(t, "test", entries[0].Name())

	entries, err = os.ReadDir(filepath.Join(destdir, "test"))
	assert.NoError(t, err)
	assert.EqualInts(t, 3, len(entries))

	for _, entry := range entries {
		assert.IsTrue(t, !entry.IsDir())
		switch entry.Name() {
		case "1", "2", "3":
			continue
		default:
			t.Fatalf("unexpected entry: %v", entry.Name())
		}
	}
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
