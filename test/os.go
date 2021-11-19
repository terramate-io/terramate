package test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
)

// WriteFile writes content to a filename inside dir directory.
// If dir is empty string then the file is created inside a temporary directory.
func WriteFile(t *testing.T, dir string, filename string, content string) string {
	t.Helper()

	if dir == "" {
		dir = t.TempDir()
	}

	path := filepath.Join(dir, filename)
	err := ioutil.WriteFile(path, []byte(content), 0700)
	assert.NoError(t, err, "writing test file %s", path)

	return path
}

// MkdirAll creates a temporary directory with default test permission bits.
func MkdirAll(t *testing.T, path string) {
	t.Helper()

	assert.NoError(t, os.MkdirAll(path, 0700), "failed to create temp directory")
}

// NonExistingDir returns a non-existing directory.
func NonExistingDir(t *testing.T) string {
	t.Helper()

	tmp := tempDir(t, "")
	tmp2 := tempDir(t, tmp)

	removeAll(t, tmp)

	return tmp2
}

func tempDir(t *testing.T, base string) string {
	t.Helper()

	dir, err := ioutil.TempDir(base, "terrastack-test")
	assert.NoError(t, err, "creating temp directory")
	return dir
}

func removeAll(t *testing.T, path string) {
	t.Helper()

	assert.NoError(t, os.RemoveAll(path), "failed to remove directory %q", path)
}
