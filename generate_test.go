package terrastack_test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terrastack"
	"github.com/mineiros-io/terrastack/test"
)

func TestGenerateFailsIfPathDoesntExist(t *testing.T) {
	assert.Error(t, terrastack.Generate(test.NonExistingDir(t)))
}
