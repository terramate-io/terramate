// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package test

import (
	"io/fs"
	stdos "os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/os"
)

// AssertTreeEquals asserts that the two given directories
// are the same. This means they must have the same files and
// also same subdirs with same files inside recursively.
func AssertTreeEquals(t *testing.T, dir1, dir2 os.Path) {
	t.Helper()

	entries1 := ReadDir(t, dir1)

	for _, entry1 := range entries1 {
		path1 := dir1.Join(entry1.Name())
		path2 := dir2.Join(entry1.Name())

		if entry1.IsDir() {
			AssertTreeEquals(t, path1, path2)
			continue
		}

		AssertFileEquals(t, path1, path2)
	}
}

// AssertFileEquals asserts that the two given files are the same.
// It assumes they are text files and shows a diff in case they are not the same.
func AssertFileEquals(t *testing.T, filepath1, filepath2 os.Path) {
	t.Helper()

	file1, err := stdos.ReadFile(filepath1.String())
	assert.NoError(t, err)

	file2, err := stdos.ReadFile(filepath2.String())
	assert.NoError(t, err)

	if diff := cmp.Diff(string(file1), string(file2)); diff != "" {
		t.Fatalf("-(%s) +(%s):\n%s", filepath1, filepath2, diff)
	}
}

// AssertFileContentEquals asserts that file fname has the content of want.
// It assumes the file content is a unicode string.
func AssertFileContentEquals(t *testing.T, fname os.Path, want string) {
	t.Helper()
	got := ReadFile(t, fname.Dir().String(), fname.Base())
	if diff := cmp.Diff(string(got), string(want)); diff != "" {
		t.Fatalf("-(%s) +(%s):\n%s", got, want, diff)
	}
}

// AssertChmod is a portable version of the os.AssertChmod.
func AssertChmod(t testing.TB, fname os.Path, mode fs.FileMode) {
	assert.NoError(t, Chmod(fname.String(), mode))
}
