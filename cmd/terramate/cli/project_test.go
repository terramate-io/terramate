package cli

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/sandbox"
	"github.com/rs/zerolog"
)

func TestLocalDefaultIsOutdated(t *testing.T) {
	s := sandbox.New(t)

	stack := s.CreateStack("stack-1")

	git := s.Git()
	git.Add(".")
	git.Commit("all")

	// dance below makes makes local main branch behind origin/main by 1 commit.
	//   - a "temp" branch is created to record current commit.
	//   - go back to main and create 1 additional commit and push to origin/main.
	//   - switch to "temp" and delete "main" reference.
	//   - create "main" branch again based on temp.

	git.CheckoutNew("temp")
	git.Checkout("main")
	stack.CreateFile("tempfile", "any content")
	git.CommitAll("additional commit")
	git.Push("main")
	git.Checkout("temp")
	git.DeleteBranch("main")
	git.CheckoutNew("main")

	prj, foundRoot, err := lookupProject(s.RootDir())

	assert.NoError(t, err)
	if !foundRoot {
		t.Fatal("unable to find root")
	}

	assert.NoError(t, prj.setDefaults(&cliSpec{}))

	g := test.NewGitWrapper(t, s.RootDir(), []string{})
	assert.IsError(t, prj.checkLocalDefaultIsUpdated(g), errors.E(ErrOutdatedLocalRev))
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
