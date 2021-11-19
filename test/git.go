package test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terrastack/git"
)

const (
	// Username for the test commits.
	Username = "terrastack tests"

	// Email for the test commits.
	Email = "terrastack@mineiros.io"
)

// NewGitWrapper tests the creation of a git wrapper and returns it if success.
func NewGitWrapper(t *testing.T, wd string, inheritEnv bool) *git.Git {
	t.Helper()

	gw, err := git.WithConfig(git.Config{
		Username:       Username,
		Email:          Email,
		WorkingDir:     wd,
		Isolated:       true,
		InheritEnv:     inheritEnv,
		AllowPorcelain: true,
	})
	assert.NoError(t, err, "new git wrapper")

	return gw
}

// EmptyRepo creates a git repository and checks for errors.
func EmptyRepo(t *testing.T) string {
	t.Helper()

	gw := NewGitWrapper(t, "", false)

	repodir := t.TempDir()
	err := gw.Init(repodir)
	assert.NoError(t, err, "git init")

	return repodir
}
