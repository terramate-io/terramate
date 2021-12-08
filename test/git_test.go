package test_test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/test"
)

func TestRepoIsSetupWithSyncRemoteOriginMain(t *testing.T) {
	const (
		remote   = "origin"
		revision = "main"
	)
	repodir := test.NewRepo(t)

	git := test.NewGitWrapper(t, repodir, false)
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
