package terrastack_test

import (
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terrastack"
	"github.com/mineiros-io/terrastack/test"
)

func TestGenerateFailsIfPathDoesntExist(t *testing.T) {
	assert.Error(t, terrastack.Generate(test.NonExistingDir(t)))
}

func TestGenerateFailsIfPathIsNotDir(t *testing.T) {
	dir := t.TempDir()
	filename := "test"

	test.WriteFile(t, dir, filename, "whatever")
	path := filepath.Join(dir, filename)

	assert.Error(t, terrastack.Generate(path))
}
