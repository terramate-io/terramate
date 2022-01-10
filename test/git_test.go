// Copyright 2021 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
