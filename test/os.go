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

package test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
)

// TempDir creates a temporary directory.
func TempDir(t testing.TB, base string) string {
	t.Helper()

	if base == "" {
		t.Fatalf("use t.TempDir() for temporary directories inside tmp")
	}

	dir, err := ioutil.TempDir(base, "terramate-test")
	assert.NoError(t, err, "creating temp directory")
	return CanonPath(t, dir)
}

// ReadDir calls os.Readir asserting the success of the operation.
func ReadDir(t testing.TB, dir string) []os.DirEntry {
	t.Helper()

	entries, err := os.ReadDir(dir)
	assert.NoError(t, err)
	return entries
}

// WriteFile writes content to a filename inside dir directory.
// If dir is empty string then the file is created inside a temporary directory.
func WriteFile(t testing.TB, dir string, filename string, content string) string {
	t.Helper()

	if dir == "" {
		dir = t.TempDir()
	}

	path := filepath.Join(dir, filename)
	pathdir := filepath.Dir(path)
	MkdirAll(t, pathdir)
	err := ioutil.WriteFile(path, []byte(content), 0700)
	assert.NoError(t, err, "writing test file %s", path)

	return path
}

// AppendFile appends content to a filename inside dir directory.
// If file exists, appends on the end of it by adding a newline,
//if file doesn't exists it will be created.
func AppendFile(t testing.TB, dir string, filename string, content string) {
	t.Helper()

	oldContent, err := os.ReadFile(filepath.Join(dir, filename))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}

	newContents := string(oldContent) + "\n" + content
	WriteFile(t, dir, filename, newContents)
}

// ReadFile reads the content of fname from dir directory.
func ReadFile(t testing.TB, dir, fname string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, fname))
	assert.NoError(t, err, "reading file")
	return data
}

// RemoveFile removes the file fname from dir directory.
// If the files doesn't exists, it succeeds.
func RemoveFile(t testing.TB, dir, fname string) {
	t.Helper()
	err := os.Remove(filepath.Join(dir, fname))
	assert.NoError(t, err)
}

// Mkdir creates a directory inside base.
func Mkdir(t testing.TB, base string, name string) string {
	path := filepath.Join(base, name)
	assert.NoError(t, os.Mkdir(path, 0700), "creating dir")
	return path
}

// MkdirAll creates a temporary directory with default test permission bits.
func MkdirAll(t testing.TB, path string) {
	t.Helper()

	assert.NoError(t, os.MkdirAll(path, 0700), "failed to create temp directory")
}

// Symlink calls [os.Symlink] failing the test if there is an error.
func Symlink(t testing.TB, oldname, newname string) {
	t.Helper()

	assert.NoError(t, os.Symlink(oldname, newname), "failed to create symlink")
}

// Getwd gets the current working dir of the process
func Getwd(t testing.TB) string {
	t.Helper()

	wd, err := os.Getwd()
	assert.NoError(t, err)
	return wd
}

// RelPath does the same as filepath.Rel but failing the test
// if an error is found.
func RelPath(t testing.TB, basepath, targetpath string) string {
	t.Helper()

	rel, err := filepath.Rel(basepath, targetpath)
	assert.NoError(t, err)
	return rel
}

// RemoveAll removes the directory and any of its children files and directories.
func RemoveAll(t testing.TB, path string) {
	t.Helper()

	assert.NoError(t, os.RemoveAll(path), "failed to remove directory %q", path)
}

// NonExistingDir returns a non-existing directory.
func NonExistingDir(t testing.TB) string {
	t.Helper()

	tmp := t.TempDir()
	tmp2 := TempDir(t, tmp)

	RemoveAll(t, tmp)

	return tmp2
}

// CanonPath returns a canonical absolute path for the given path.
// Fails the test if any error is found.
func CanonPath(t testing.TB, path string) string {
	t.Helper()

	p, err := filepath.EvalSymlinks(path)
	assert.NoError(t, err)
	p, err = filepath.Abs(p)
	assert.NoError(t, err)
	return p
}
