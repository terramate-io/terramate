// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package sandbox_test

import (
	"testing"

	"github.com/terramate-io/terramate/test/sandbox"
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
