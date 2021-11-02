package git_test

import (
	"errors"
	"os"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terrastack/git"
	"github.com/mineiros-io/terrastack/test"
)

const CookedCommitID = "877f9b30fe7ff224bdd38b7d04b87d50a2e0fcd8"

func TestGit(t *testing.T) {
	git, err := git.NewWrapper(test.Username, test.Email)
	assert.NoError(t, err, "new git wrapper")

	version, err := git.Version()
	assert.NoError(t, err, "git version")

	t.Logf("git version: %s", version)
}

func TestGitLog(t *testing.T) {
	type testcase struct {
		repo    func(t *testing.T) string
		revs    []string
		want    []git.LogLine
		wantErr error
	}

	var removeRepos []string

	defer func() {
		for _, d := range removeRepos {
			os.RemoveAll(d)
		}
	}()

	for _, tc := range []testcase{
		{
			repo: mkOneCommitRepo,
			revs: []string{"HEAD"},
			want: []git.LogLine{
				{
					CommitID: CookedCommitID,
					Message:  "some message",
				},
			},
			wantErr: nil,
		},
		{
			repo: mkOneCommitRepo,
			revs: []string{"main"},
			want: []git.LogLine{
				{
					CommitID: CookedCommitID,
					Message:  "some message",
				},
			},
			wantErr: nil,
		},
		{
			repo: mkOneCommitRepo,
			revs: []string{"main", "HEAD"},
			want: []git.LogLine{
				{
					CommitID: CookedCommitID,
					Message:  "some message",
				},
			},
			wantErr: nil,
		},
		{
			repo:    mkOneCommitRepo,
			revs:    []string{"^HEAD"},
			want:    []git.LogLine{},
			wantErr: nil,
		},
		{
			repo: mkOneCommitRepo,
			revs: []string{"non-existent-branch"},
			want: []git.LogLine{},

			// we only check if error Is of type CmdError then the state do not
			// matter.
			wantErr: git.NewCmdError("any command", nil, nil),
		},
	} {
		repodir := tc.repo(t)

		removeRepos = append(removeRepos, repodir)

		gw, err := git.WithConfig(git.Config{
			WorkingDir: repodir,
		})
		assert.NoError(t, err, "new git wrapper")

		logs, err := gw.LogSummary(tc.revs...)

		if tc.wantErr != nil {
			if err == nil {
				t.Errorf("expected error: %v", err)
				return
			}

			if !errors.Is(err, tc.wantErr) {
				t.Errorf("error type mismatch: want[%s] but got [%s]",
					tc.wantErr, err)
			}
		}

		assert.EqualInts(t, len(tc.want), len(logs), "log count mismatch")

		for i := 0; i < len(tc.want); i++ {
			assert.EqualStrings(t, tc.want[i].CommitID, logs[i].CommitID,
				"log commitid mismatch: %s != %s",
				tc.want[i].CommitID, logs[i].CommitID)

			assert.EqualStrings(t, tc.want[i].Message, logs[i].Message,
				"log message mismatch: %s != %s",
				tc.want[i].Message, logs[i].Message)
		}
	}
}

func TestRevParse(t *testing.T) {
	repodir := mkOneCommitRepo(t)
	defer os.RemoveAll(repodir)

	git := test.NewGitWrapper(t, repodir, false)

	out, err := git.RevParse("main")
	assert.NoError(t, err, "rev-parse failed")
	assert.EqualStrings(t, CookedCommitID, out, "commit mismatch")
}

func mkOneCommitRepo(t *testing.T) string {
	repodir := test.EmptyRepo(t)

	// Fixing all the information used to create the SHA-1 below:
	// CommitID: 877f9b30fe7ff224bdd38b7d04b87d50a2e0fcd8

	os.Setenv("GIT_COMMITTER_DATE", "1597490918 +0530")
	os.Setenv("GIT_AUTHOR_DATE", "1597490918 +0530")
	os.Setenv("GIT_COMMITTER_NAME", test.Username)
	os.Setenv("GIT_AUTHOR_NAME", test.Username)
	os.Setenv("GIT_COMMITTER_EMAIL", test.Email)
	os.Setenv("GIT_AUTHOR_EMAIL", test.Email)

	defer func() {
		os.Unsetenv("GIT_COMMITTER_DATE")
		os.Unsetenv("GIT_AUTHOR_DATE")
		os.Unsetenv("GIT_COMMITTER_NAME")
		os.Unsetenv("GIT_AUTHOR_NAME")
		os.Unsetenv("GIT_COMMITTER_EMAIL")
		os.Unsetenv("GIT_AUTHOR_EMAIL")
	}()

	gw := test.NewGitWrapper(t, repodir, true)

	filename := test.CreateFile(t, repodir, "README.md", "# Test")

	assert.NoError(t, gw.Add(filename), "git add %s", filename)

	err := gw.Commit("some message")
	assert.NoError(t, err, "commit")

	return repodir
}
