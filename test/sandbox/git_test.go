// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package sandbox_test

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestInitializedGitHasOriginMain(t *testing.T) {
	t.Parallel()
	basedir := test.TempDir(t)
	git := sandbox.NewGit(t, basedir)
	git.Init()
	git.RevParse("origin/main")
}

func TestInitializeArbitraryRemote(t *testing.T) {
	t.Parallel()
	basedir := test.TempDir(t)
	git := sandbox.NewGit(t, basedir)
	git.InitLocalRepo()

	// the main branch only exists after first commit.
	path := test.WriteFile(t, git.BaseDir(), "README.md", "# generated by terramate")
	git.Add(path.String())
	git.Commit("first commit")

	const remote = "mineiros"
	const remoteBranch = "default"

	git.SetupRemote(remote, remoteBranch, "main")
	git.RevParse(remote + "/" + remoteBranch)
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
