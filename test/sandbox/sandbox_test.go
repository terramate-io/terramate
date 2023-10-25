// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package sandbox_test

import (
	"testing"

	"github.com/terramate-io/terramate/test/sandbox"
)

func TestSandboxWithArbitraryRepoConfig(t *testing.T) {
	t.Parallel()
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
