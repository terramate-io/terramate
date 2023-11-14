// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
)

var tmTestRootTempdir string

func init() {
	tmTestRootTempdir = os.Getenv("TM_TEST_ROOT_TEMPDIR")
}

// TempDir creates a temporary directory.
func TempDir(t testing.TB) string {
	t.Helper()
	if tmTestRootTempdir == "" {
		// fallback for the slower implementation if env is not set.
		return t.TempDir()
	}
	return tempDir(t, tmTestRootTempdir)
}

// DoesNotExist calls os.Stat and asserts that the entry does not exist
func DoesNotExist(t testing.TB, dir, fname string) {
	t.Helper()
	_, err := os.Stat(filepath.Join(dir, fname))
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	assert.NoError(t, err, "stat error")

	t.Fatalf("should not exist: %s", fname)
}

// IsDir calls os.Stat and asserts that the entry is a directory
func IsDir(t testing.TB, dir, fname string) {
	t.Helper()
	isDirOrFile(t, dir, fname, true)
}

// IsFile calls os.Stat and asserts that the entry is a file
func IsFile(t testing.TB, dir, fname string) {
	t.Helper()
	isDirOrFile(t, dir, fname, false)
}

func isDirOrFile(t testing.TB, dir, fname string, isDir bool) {
	t.Helper()
	fi, err := os.Stat(filepath.Join(dir, fname))
	if errors.Is(err, os.ErrNotExist) {
		if isDir {
			t.Fatalf("directory does not exist: %s", fname)
		} else {
			t.Fatalf("file does not exist: %s", fname)
		}
		return
	}
	assert.NoError(t, err, "stat error")

	assert.IsTrue(t, fi.IsDir() == isDir, "want:\n%s\ngot:\n%s\n", fi.IsDir(), isDir)
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
		dir = TempDir(t)
	}

	path := filepath.Join(dir, filename)
	pathdir := filepath.Dir(path)
	MkdirAll(t, pathdir)
	err := os.WriteFile(path, []byte(content), 0700)
	assert.NoError(t, err, "writing test file %s", path)

	return path
}

// AppendFile appends content to a filename inside dir directory.
// If file exists, appends on the end of it by adding a newline,
// if file doesn't exists it will be created.
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

// MkdirAll2 creates a temporary directory with provided permissions.
func MkdirAll2(t testing.TB, path string, perm fs.FileMode) {
	t.Helper()

	assert.NoError(t, os.MkdirAll(path, perm), "failed to create temp directory")
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

	tmp := TempDir(t)
	tmp2 := tempDir(t, tmp)

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

// PrependToPath prepend a directory to the OS PATH variable in a portable way.
// It returns the new env slice and a boolean telling if the env was updated or
// not.
func PrependToPath(env []string, dir string) ([]string, bool) {
	envKeyEquality := func(s1, s2 string) bool { return s1 == s2 }
	if runtime.GOOS == "windows" {
		envKeyEquality = strings.EqualFold
	}

	for i, v := range env {
		eqPos := strings.Index(v, "=")
		key := v[:eqPos]
		oldv := v[eqPos+1:]
		if envKeyEquality(key, "PATH") {
			v = key + "=" + dir + string(os.PathListSeparator) + oldv
			env[i] = v
			return env, true
		}
	}
	return env, false
}

func tempDir(t testing.TB, base string) string {
	dir, err := os.MkdirTemp(base, "terramate-test")
	assert.NoError(t, err, "creating temp directory")
	return dir
}
