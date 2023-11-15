// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build linux || darwin

package git_test

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/madlambda/spells/assert"
	"github.com/rs/zerolog"
	"github.com/terramate-io/terramate/git"
	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/sandbox"
)

const CookedCommitID = "4e991b55e3d58b9c3137a791a9986ed9c5069697"

func TestGit(t *testing.T) {
	t.Parallel()
	git, err := git.WithConfig(git.Config{})
	assert.NoError(t, err, "new git wrapper")

	version, err := git.Version()
	assert.NoError(t, err, "git version")

	t.Logf("git version: %s", version)
}

func TestGitLog(t *testing.T) {
	t.Parallel()
	type testcase struct {
		repo    func(t *testing.T) string
		revs    []string
		want    []git.LogLine
		wantErr error
	}

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
	t.Parallel()
	repodir := mkOneCommitRepo(t)

	git := test.NewGitWrapper(t, repodir, []string{})
	out, err := git.RevParse("main")
	assert.NoError(t, err, "rev-parse failed")
	assert.EqualStrings(t, CookedCommitID, out, "commit mismatch")
}

func TestGitOptions(t *testing.T) {
	t.Parallel()
	repodir1 := mkOneCommitRepo(t)
	repodir2 := mkOneCommitRepo(t)

	git := test.NewGitWrapper(t, repodir1, []string{})
	gotRepoDir1, err := git.Root()
	assert.NoError(t, err, "root failed")
	assert.EqualStrings(t, repodir1, gotRepoDir1)
	gotRepoDir2, err := git.With().WorkingDir(repodir2).Wrapper().Root()
	assert.NoError(t, err)
	assert.EqualStrings(t, repodir2, gotRepoDir2)
}

func TestClone(t *testing.T) {
	const (
		filename = "test.txt"
		content  = "test"
	)
	s := sandbox.New(t)
	s.RootEntry().CreateFile(filename, content)
	git := s.Git()

	git.CommitAll("add file")

	repoURL := "file://" + s.RootDir()
	cloneDir := test.TempDir(t)
	git.Clone(repoURL, cloneDir)

	got := test.ReadFile(t, cloneDir, filename)
	assert.EqualStrings(t, content, string(got))
}

func TestCurrentBranch(t *testing.T) {
	t.Parallel()
	s := sandbox.New(t)
	git := s.Git()

	assert.EqualStrings(t, "main", git.CurrentBranch())

	const newBranch = "test"

	git.CheckoutNew(newBranch)
	assert.EqualStrings(t, newBranch, git.CurrentBranch())
}

func TestFetchRemoteRev(t *testing.T) {
	t.Parallel()
	repodir := mkOneCommitRepo(t)
	git := test.NewGitWrapper(t, repodir, []string{})

	remote, revision := addDefaultRemoteRev(t, git)

	remoteRef, err := git.FetchRemoteRev(remote, revision)
	assert.NoError(t, err, "git.FetchRemoteRev(%q, %q)", remote, revision)

	assert.EqualStrings(
		t,
		CookedCommitID,
		remoteRef.CommitID,
		"remote reference ID doesn't match cooked commit ID",
	)

	const wantRefName = "refs/heads/main"

	assert.EqualStrings(
		t,
		wantRefName,
		remoteRef.Name,
		"remote ref name doesn't match local",
	)

}

func TestFetchRemoteRevErrorHandling(t *testing.T) {
	t.Parallel()
	repodir := mkOneCommitRepo(t)
	git := test.NewGitWrapper(t, repodir, []string{})
	// should fail because the repo has no origin remote set.
	remoteRef, err := git.FetchRemoteRev("origin", "main")
	assert.Error(t, err, "unexpected result: %v", remoteRef)
}

func TestListingAvailableRemotes(t *testing.T) {
	t.Parallel()
	type testcase struct {
		name string
		want []git.Remote
	}

	tests := []testcase{
		{
			name: "no remotes",
		},
		{
			name: "one remote",
			want: []git.Remote{
				{
					Name:     "origin",
					Branches: []string{"main"},
				},
			},
		},
		{
			name: "two branches",
			want: []git.Remote{
				{
					Name:     "origin",
					Branches: []string{"main", "test"},
				},
			},
		},
		{
			name: "branches with one forward slash",
			want: []git.Remote{
				{
					Name:     "origin",
					Branches: []string{"main", "test/hi"},
				},
			},
		},
		{
			name: "branches with multiple forward slashes",
			want: []git.Remote{
				{
					Name:     "origin",
					Branches: []string{"main", "test/hi/one/more/yay"},
				},
			},
		},
		{
			name: "two remotes",
			want: []git.Remote{
				{
					Name:     "another",
					Branches: []string{"main"},
				},
				{
					Name:     "origin",
					Branches: []string{"main"},
				},
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repodir := mkOneCommitRepo(t)
			g := test.NewGitWrapper(t, repodir, []string{})

			for _, gitRemote := range tc.want {

				remote := gitRemote.Name
				remoteDir := test.EmptyRepo(t, true)
				err := g.RemoteAdd(remote, remoteDir)
				assert.NoError(t, err)

				for _, branch := range gitRemote.Branches {

					if branch == defaultBranch {
						err = g.Push(remote, branch)
						assert.NoError(t, err)
						continue
					}

					assert.NoError(t, g.Checkout(branch, true))
					assert.NoError(t, g.Push(remote, branch))
					assert.NoError(t, g.Checkout(defaultBranch, false))
				}
			}

			gotRemotes, err := g.Remotes()
			assert.NoError(t, err)

			assertEqualRemotes(t, gotRemotes, tc.want)
		})
	}

}

func TestListRemoteWithMultipleBranches(t *testing.T) {
	t.Parallel()
	const (
		remote = "origin"
	)

	repodir := mkOneCommitRepo(t)
	g := test.NewGitWrapper(t, repodir, []string{})

	remoteDir := test.EmptyRepo(t, true)

	assert.NoError(t, g.RemoteAdd(remote, remoteDir))
	assert.NoError(t, g.Push(remote, defaultBranch))

	branches := []string{"b1", "b2", "b3"}
	for _, branch := range branches {
		assert.NoError(t, g.Checkout(branch, true))
		assert.NoError(t, g.Push(remote, branch))
	}

	got, err := g.Remotes()
	assert.NoError(t, err)

	want := []git.Remote{
		{
			Name:     remote,
			Branches: append(branches, defaultBranch),
		},
	}

	assertEqualRemotes(t, got, want)
}

func TestShowMetadata(t *testing.T) {
	type testcase struct {
		name        string
		title       string
		description string
	}

	tests := []testcase{
		{
			name:        "title-only",
			title:       "add feature x",
			description: "",
		},
		{
			name:        "title with single-line description",
			title:       "add feature y",
			description: "this is the latest feature",
		},
		{
			name:  "title with multi-line description",
			title: "add feature y",
			description: `this is the latest feature:
				* a
				* b
				* c`,
		},
	}

	for _, tc := range tests {
		repodir := test.EmptyRepo(t, false)

		env := []string{
			"GIT_COMMITTER_DATE=1597490918 +0530",
			"GIT_AUTHOR_DATE=1597490918 +0530",
			"GIT_COMMITTER_NAME=" + test.Username,
			"GIT_AUTHOR_NAME=" + test.Username,
			"GIT_COMMITTER_EMAIL=" + test.Email,
			"GIT_AUTHOR_EMAIL=" + test.Email,
		}

		commitTime := time.Unix(1597490918, 0)

		gw := test.NewGitWrapper(t, repodir, env)
		filename := test.WriteFile(t, repodir, "README.md", "# Test")
		assert.NoError(t, gw.Add(filename), "git add %s", filename)

		commitMsg := tc.title
		if tc.description != "" {
			// git commit message uses two newlines to separate subject and body
			commitMsg += "\n\n" + tc.description
		}

		err := gw.Commit(commitMsg)
		assert.NoError(t, err, "commit")

		got, err := gw.ShowCommitMetadata("HEAD")
		assert.NoError(t, err)

		want := &git.CommitMetadata{
			Author:  test.Username,
			Email:   test.Email,
			Time:    &commitTime,
			Subject: tc.title,
			Body:    tc.description,
		}

		if diff := cmp.Diff(got, want); diff != "" {
			t.Fatalf(
				"failed test '%s', got metadata %v != want %v. Details (got-, want+):\n%s",
				tc.name,
				got,
				want,
				diff,
			)
		}
	}
}

func TestGetConfigValue(t *testing.T) {
	repodir := mkOneCommitRepo(t)

	gw, err := git.WithConfig(git.Config{
		Username:       test.Username,
		Email:          test.Email,
		WorkingDir:     repodir,
		Isolated:       true,
		Env:            []string{},
		AllowPorcelain: true,
		GlobalArgs:     []string{"-c", fmt.Sprintf("safe.directory=%s", repodir)},
	})
	assert.NoError(t, err, "new git wrapper")

	// Existing values
	tests := map[string]string{
		"user.name":      test.Username,
		"user.email":     test.Email,
		"safe.directory": repodir,
	}

	for k, v := range tests {
		out, err := gw.GetConfigValue(k)
		assert.NoError(t, err, "git config %s", k)
		assert.EqualStrings(t, v, out)
	}

	// Non-existing value
	_, err = gw.GetConfigValue("nothing")
	assert.Error(t, err, "git config: non-existing key")
}

func TestFindNearestCommonParent(t *testing.T) {
	repodir := test.EmptyRepo(t, false)

	gw, err := git.WithConfig(git.Config{
		WorkingDir:     repodir,
		Isolated:       true,
		AllowPorcelain: true,
	})
	assert.NoError(t, err, "new git wrapper")

	hashToName := map[string]string{"": ""}
	nameToHash := map[string]string{"": ""}

	setNamedCommit := func(name string) {
		hash, err := gw.RevParse("HEAD")
		assert.NoError(t, err, "git rev-parse")

		hashToName[hash] = name
		nameToHash[name] = hash
	}

	makeNamedCommit := func(name string) {
		test.WriteFile(t, repodir, name, "")
		assert.NoError(t, gw.Add(name), "git add")
		assert.NoError(t, gw.Commit(name), "git commit")

		setNamedCommit(name)
	}

	type testcase struct {
		Commit     string
		Ref        string
		ForkedFrom string

		WantForkPoint    string
		WantIsMainCommit bool
	}

	var tests []testcase

	makeNamedCommit("main_commit_1")

	tests = append(tests, []testcase{
		{
			Commit:           "main_commit_1",
			ForkedFrom:       "main",
			WantForkPoint:    "main_commit_1",
			WantIsMainCommit: true,
		}}...,
	)

	assert.NoError(t, gw.Checkout("branch_a", true))
	makeNamedCommit("branch_a_commit_1")
	makeNamedCommit("branch_a_commit_2")

	tests = append(tests, []testcase{
		{
			Commit:        "branch_a_commit_1",
			ForkedFrom:    "main",
			WantForkPoint: "main_commit_1",
		},
		{
			Commit:        "branch_a_commit_2",
			ForkedFrom:    "main",
			WantForkPoint: "main_commit_1",
		},
		{
			Ref:           "branch_a",
			ForkedFrom:    "main",
			WantForkPoint: "main_commit_1",
		}}...,
	)

	assert.NoError(t, gw.Checkout("main", false))
	assert.NoError(t, gw.Merge("branch_a"))
	setNamedCommit("main_commit_2")

	tests = append(tests, []testcase{
		{
			Commit:           "main_commit_2",
			ForkedFrom:       "main",
			WantForkPoint:    "main_commit_2",
			WantIsMainCommit: true,
		}}...,
	)

	assert.NoError(t, gw.Checkout("branch_b", true))
	makeNamedCommit("branch_b_commit_1")
	makeNamedCommit("branch_b_commit_2")

	tests = append(tests, []testcase{
		{
			ForkedFrom:    "main",
			Commit:        "branch_b_commit_1",
			WantForkPoint: "main_commit_2",
		},
		{
			ForkedFrom:    "main",
			Commit:        "branch_b_commit_2",
			WantForkPoint: "main_commit_2",
		},
		{
			Ref:           "branch_b",
			ForkedFrom:    "main",
			WantForkPoint: "main_commit_2",
		}}...,
	)

	assert.NoError(t, gw.Checkout("main", false))
	assert.NoError(t, gw.Merge("branch_b"))
	setNamedCommit("main_commit_3")

	tests = append(tests, []testcase{
		{
			Commit:           "main_commit_3",
			ForkedFrom:       "main",
			WantForkPoint:    "main_commit_3",
			WantIsMainCommit: true,
		}}...,
	)

	assert.NoError(t, gw.Checkout("branch_unmerged", true))
	makeNamedCommit("branch_unmerged_commit_1")
	makeNamedCommit("branch_unmerged_commit_2")
	makeNamedCommit("branch_unmerged_commit_3")

	tests = append(tests, []testcase{
		{
			Commit:        "branch_unmerged_commit_1",
			ForkedFrom:    "main",
			WantForkPoint: "main_commit_3",
		},
		{
			Commit:        "branch_unmerged_commit_2",
			ForkedFrom:    "main",
			WantForkPoint: "main_commit_3",
		},
		{
			Commit:        "branch_unmerged_commit_3",
			ForkedFrom:    "main",
			WantForkPoint: "main_commit_3",
		},
		{
			Ref:           "branch_unmerged",
			ForkedFrom:    "main",
			WantForkPoint: "main_commit_3",
		}}...,
	)

	assert.NoError(t, gw.Checkout("main", false))
	assert.NoError(t, gw.Checkout("branch_c", true))
	makeNamedCommit("branch_c_commit_1")

	assert.NoError(t, gw.Checkout("branch_d", true))
	makeNamedCommit("branch_d_commit_1")

	assert.NoError(t, gw.Checkout("branch_c", false))
	assert.NoError(t, gw.Merge("branch_d"))
	setNamedCommit("branch_c_commit_2")

	makeNamedCommit("branch_c_commit_3")

	assert.NoError(t, gw.Checkout("main", false))
	assert.NoError(t, gw.Merge("branch_c"))
	setNamedCommit("main_commit_4")

	tests = append(tests, []testcase{
		{
			Commit:        "branch_c_commit_1",
			ForkedFrom:    "main",
			WantForkPoint: "main_commit_3",
		},
		{
			Commit:        "branch_d_commit_1",
			ForkedFrom:    "main",
			WantForkPoint: "main_commit_3",
		},
		{
			Commit:        "branch_d_commit_1",
			ForkedFrom:    "branch_c",
			WantForkPoint: "branch_c_commit_1",
		},
		{
			Commit:        "branch_c_commit_2",
			ForkedFrom:    "main",
			WantForkPoint: "main_commit_3",
		},
		{
			Commit:        "branch_c_commit_3",
			ForkedFrom:    "main",
			WantForkPoint: "main_commit_3",
		},
		{
			Commit:           "main_commit_4",
			ForkedFrom:       "main",
			WantForkPoint:    "main_commit_4",
			WantIsMainCommit: true,
		},
		{
			Ref:           "branch_d",
			ForkedFrom:    "branch_c",
			WantForkPoint: "branch_c_commit_1",
		},
		{
			Ref:           "branch_d",
			ForkedFrom:    "main",
			WantForkPoint: "main_commit_3",
		},
		{
			Ref:           "branch_c",
			ForkedFrom:    "main",
			WantForkPoint: "main_commit_3",
		},
		{
			Ref:           "branch_c",
			ForkedFrom:    "branch_d",
			WantForkPoint: "branch_d_commit_1",
		}}...,
	)

	assert.NoError(t, gw.Checkout("branch_wip", true))
	makeNamedCommit("branch_wip_commit_1")
	makeNamedCommit("branch_wip_commit_2")

	tests = append(tests, []testcase{
		{
			Commit:        "branch_wip_commit_1",
			ForkedFrom:    "main",
			WantForkPoint: "main_commit_4",
		},
		{
			Commit:        "branch_wip_commit_2",
			ForkedFrom:    "main",
			WantForkPoint: "main_commit_4",
		},
		{
			Ref:           "branch_wip",
			ForkedFrom:    "main",
			WantForkPoint: "main_commit_4",
		}}...,
	)

	/*
		  * branch_wip_commit_2
		  * branch_wip_commit_1
		 /
		* main_commit_4
		|\
		| * branch_c_commit_3
		| * branch_c_commit_2
		| |\
		| | * branch_d_commit_1
		| |/
		| * branch_c_commit_1
		|/
		|
		| * branch_unmerged_commit_3
		| * branch_unmerged_commit_2
		| * branch_unmerged_commit_1
		|/
		* main_commit_3
		|\
		| * branch_b_commit_2
		| * branch_b_commit_1
		|/
		* main_commit_2
		|\
		| * branch_a_commit_2
		| * branch_a_commit_1
		|/
		* main_commit_1
	*/

	for _, tc := range tests {
		assert.IsTrue(t, (tc.Commit != "") != (tc.Ref != ""), "set either commit or ref")
		var rev string
		if tc.Commit != "" {
			rev = nameToHash[tc.Commit]
		} else {
			rev = tc.Ref
		}

		wantName := tc.WantForkPoint
		wantHash := nameToHash[wantName]

		gotHash, err := gw.FindNearestCommonParent(tc.ForkedFrom, rev)
		assert.NoError(t, err, "FindNearestCommonParent")
		gotName := hashToName[gotHash]

		assert.EqualStrings(t, wantHash, gotHash,
			"fork point for %v (%v), want = %v (%v), got = %v (%v)",
			tc.Commit, rev, wantName, wantHash, gotName, gotHash)

		gotIsMainCommit, err := gw.IsFirstParentAncestor(rev, "main")
		assert.NoError(t, err, "IsFirstParentAncestor")

		if tc.WantIsMainCommit {
			assert.IsTrue(t, gotIsMainCommit,
				"%v (%v) is main commit", tc.Commit, rev)
		} else {
			assert.IsTrue(t, !gotIsMainCommit,
				"%v (%v) is not main commit", tc.Commit, rev)
		}

	}
}

const defaultBranch = "main"

func mkOneCommitRepo(t *testing.T) string {
	dir := test.EmptyRepo(t, false)
	repodir, err := filepath.EvalSymlinks(dir)
	assert.NoError(t, err)

	// Fixing all the information used to create the SHA-1 below:
	// CommitID: a022c39b57b1e711fb9298a05aacc699773e6d36

	// Other than the environment variables below, the file's permission bits
	// are also used as entropy for the commitid.
	env := []string{
		"GIT_COMMITTER_DATE=1597490918 +0530",
		"GIT_AUTHOR_DATE=1597490918 +0530",
		"GIT_COMMITTER_NAME=" + test.Username,
		"GIT_AUTHOR_NAME=" + test.Username,
		"GIT_COMMITTER_EMAIL=" + test.Email,
		"GIT_AUTHOR_EMAIL=" + test.Email,
	}

	gw := test.NewGitWrapper(t, repodir, env)
	filename := test.WriteFile(t, repodir, "README.md", "# Test")
	assert.NoError(t, gw.Add(filename), "git add %s", filename)

	err = gw.Commit("some message")
	assert.NoError(t, err, "commit")

	return repodir
}

func addDefaultRemoteRev(t *testing.T, git *git.Git) (string, string) {
	const (
		remote   = "origin"
		revision = "main"
	)
	t.Helper()

	remoteDir := test.EmptyRepo(t, true)
	err := git.RemoteAdd(remote, remoteDir)
	assert.NoError(t, err)

	err = git.Push(remote, revision)
	assert.NoError(t, err)

	return remote, revision
}

func assertEqualRemotes(t *testing.T, got []git.Remote, want []git.Remote) {
	t.Helper()

	if diff := cmp.Diff(got, want); diff != "" {
		t.Fatalf(
			"got remotes %v != want %v. Details (got-, want+):\n%s",
			got,
			want,
			diff,
		)
	}
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
