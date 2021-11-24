package test_test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terrastack/test"
)

func TestRepo(t *testing.T) {
	repodir := test.NewRepo(t)

	gw := test.NewGitWrapper(t, repodir, false)
	_, err := gw.RevParse("origin/main")
	assert.NoError(t, err, "new repo must resolve origin/main")
}
