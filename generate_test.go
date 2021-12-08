package terramate_test

import (
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate"
	"github.com/mineiros-io/terramate/test"
)

func TestGenerateFailsIfPathDoesntExist(t *testing.T) {
	assert.Error(t, terramate.Generate(test.NonExistingDir(t)))
}

func TestGenerateFailsIfPathIsNotDir(t *testing.T) {
	dir := t.TempDir()
	filename := "test"

	test.WriteFile(t, dir, filename, "whatever")
	path := filepath.Join(dir, filename)

	assert.Error(t, terramate.Generate(path))
}

func TestGenerateFailsIfPathIsRelative(t *testing.T) {
	dir := t.TempDir()
	relpath := test.RelPath(t, test.Getwd(t), dir)

	assert.Error(t, terramate.Generate(relpath))
}
