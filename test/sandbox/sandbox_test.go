package sandbox_test

import (
	"testing"

	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestSandboxWithArbitraryRepoConfig(t *testing.T) {
	const remote = "mineiros"
	const remoteBranch = "default"
	const localBranch = "trunk"

	s := sandbox.NewWithGitConfig(t, sandbox.GitConfig{
		LocalBranchName:         localBranch,
		DefaultRemoteName:       remote,
		DefaultRemoteBranchName: remoteBranch,
	})

	git := s.Git()
	git.RevParse(localBranch)
	git.RevParse(remote + "/" + remoteBranch)
}
