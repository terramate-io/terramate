package sandbox_test

import (
	"testing"

	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestInitializedGitHasOriginMain(t *testing.T) {
	basedir := t.TempDir()
	git := sandbox.NewGit(t, basedir)
	git.Init()
	git.RevParse("origin/main")
}
