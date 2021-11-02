package test

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
)

// TempDir creates a temporary directory.
func TempDir(t *testing.T, base string) string {
	t.Helper()

	dir, err := ioutil.TempDir(base, "terrastack-test")
	assert.NoError(t, err, "creating temp directory")
	return dir
}

// CreateFile creates a file inside dir directory with provided content.
// If dir is empty string then it also creates a temporary directory for it.
func CreateFile(t *testing.T, dir string, filename string, content string) string {
	t.Helper()

	if dir == "" {
		dir = TempDir(t, "")
	}

	path := filepath.Join(dir, filename)
	err := ioutil.WriteFile(path, []byte(content), 0644)
	assert.NoError(t, err, "writing test file %s", path)

	return path
}
