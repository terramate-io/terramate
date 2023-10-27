// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package test_test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/rs/zerolog"
	"github.com/terramate-io/terramate/test"
)

func TestRepoIsSetupWithSyncRemoteOriginMain(t *testing.T) {
	t.Parallel()
	const (
		remote   = "origin"
		revision = "main"
	)
	repodir := test.NewRepo(t)

	git := test.NewGitWrapper(t, repodir, []string{})
	originMainRev := remote + "/" + revision
	commitID, err := git.RevParse(originMainRev)
	assert.NoError(t, err, "git.RevParse(%q)", originMainRev)

	remoteRef, err := git.FetchRemoteRev(remote, revision)
	assert.NoError(t, err, "git.FetchRemoteRev(%q, %q)", remote, revision)

	assert.EqualStrings(
		t,
		commitID,
		remoteRef.CommitID,
		"%q remote rev doesn't match local",
		originMainRev,
	)
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
