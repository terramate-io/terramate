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

package test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/madlambda/spells/assert"
)

// AssertTreeEquals asserts that the two given directories
// are the same. This means they must have the same files and
// also same subdirs with same files inside recursively.
func AssertTreeEquals(t *testing.T, dir1, dir2 string) {
	t.Helper()

	entries1 := ReadDir(t, dir1)

	for _, entry1 := range entries1 {
		path1 := filepath.Join(dir1, entry1.Name())
		path2 := filepath.Join(dir2, entry1.Name())

		if entry1.IsDir() {
			AssertTreeEquals(t, path1, path2)
			continue
		}

		AssertFileEquals(t, path1, path2)
	}
}

// AssertFileEquals asserts that the two given files are the same.
// It assumes they are text files and shows a diff in case they are not the same.
func AssertFileEquals(t *testing.T, filepath1, filepath2 string) {
	t.Helper()

	file1, err := os.ReadFile(filepath1)
	assert.NoError(t, err)

	file2, err := os.ReadFile(filepath2)
	assert.NoError(t, err)

	if diff := cmp.Diff(string(file1), string(file2)); diff != "" {
		t.Fatalf("-(%s) +(%s):\n%s", filepath1, filepath2, diff)
	}
}

// AssertFileContentEquals asserts that file fname has the content of want.
// It assumes the file content is a unicode string.
func AssertFileContentEquals(t *testing.T, fname string, want string) {
	t.Helper()
	got := ReadFile(t, filepath.Dir(fname), filepath.Base(fname))
	if diff := cmp.Diff(string(got), string(want)); diff != "" {
		t.Fatalf("-(%s) +(%s):\n%s", got, want, diff)
	}
}

func Chmod(t *testing.T, fname string, mode fs.FileMode) {
	assert.NoError(t, chmod(fname, mode))
}
