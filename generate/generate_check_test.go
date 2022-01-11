package generate_test

import (
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/generate"
	"github.com/mineiros-io/terramate/test"
)

func TestCheckFailsIfPathDoesntExist(t *testing.T) {
	_, err := generate.Check(test.NonExistingDir(t))
	assert.Error(t, err)
}

func TestCheckFailsIfPathIsNotDir(t *testing.T) {
	dir := t.TempDir()
	filename := "test"

	test.WriteFile(t, dir, filename, "whatever")
	path := filepath.Join(dir, filename)

	_, err := generate.Check(path)
	assert.Error(t, err)
}

func TestCheckFailsIfPathIsRelative(t *testing.T) {
	dir := t.TempDir()
	relpath := test.RelPath(t, test.Getwd(t), dir)

	_, err := generate.Check(relpath)
	assert.Error(t, err)
}
