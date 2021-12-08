package test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
)

// TempDir creates a temporary directory.
func TempDir(t *testing.T, base string) string {
	t.Helper()

	if base == "" {
		t.Fatalf("use t.TempDir() for temporary directories inside tmp")
	}

	dir, err := ioutil.TempDir(base, "terramate-test")
	assert.NoError(t, err, "creating temp directory")
	return dir
}

// WriteFile writes content to a filename inside dir directory.
// If dir is empty string then the file is created inside a temporary directory.
func WriteFile(t *testing.T, dir string, filename string, content string) string {
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

// ReadFile reads the content of fname from dir directory.
func ReadFile(t *testing.T, dir, fname string) []byte {
	data, err := os.ReadFile(filepath.Join(dir, fname))
	assert.NoError(t, err, "reading file")
	return data
}

// Mkdir creates a directory inside base.
func Mkdir(t *testing.T, base string, name string) string {
	path := filepath.Join(base, name)
	assert.NoError(t, os.Mkdir(path, 0700), "creating dir")
	return path
}

// MkdirAll creates a temporary directory with default test permission bits.
func MkdirAll(t *testing.T, path string) {
	t.Helper()

	assert.NoError(t, os.MkdirAll(path, 0700), "failed to create temp directory")
}

// RemoveAll removes the directory and any of its children files and directories.
func RemoveAll(t *testing.T, path string) {
	t.Helper()

	assert.NoError(t, os.RemoveAll(path), "failed to remove directory %q", path)
}

// NonExistingDir returns a non-existing directory.
func NonExistingDir(t *testing.T) string {
	t.Helper()

	tmp := t.TempDir()
	tmp2 := TempDir(t, tmp)

	RemoveAll(t, tmp)

	return tmp2
}
