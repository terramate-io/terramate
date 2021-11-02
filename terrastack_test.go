package terrastack_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terrastack"
	"github.com/mineiros-io/terrastack/test"
)

func TestListStacks(t *testing.T) {
	type testcase struct {
		basedir func(t *testing.T) string
		want    []string
		wantErr error
	}

	allstacks := []string{}

	defer func() {
		for _, d := range allstacks {
			removeStack(t, d)
		}
	}()

	for _, tc := range []testcase{
		{
			basedir: nonExistentDir,
			wantErr: os.ErrNotExist,
		},
		{
			basedir: singleStack,
			want:    []string{"/"},
			wantErr: nil,
		},
		{
			basedir: subStack,
			want:    []string{"/", "/substack"},
			wantErr: nil,
		},
		{
			basedir: nestedStacks,
			want:    []string{"/", "/substack", "/substack/deepstack"},
			wantErr: nil,
		},
	} {
		basedir := tc.basedir(t)

		allstacks = append(allstacks, basedir)

		stacks, err := terrastack.List(basedir)

		if tc.wantErr != nil {
			if err == nil {
				t.Errorf("expected error: %v", tc.wantErr)
			}

			if !errors.Is(err, tc.wantErr) {
				t.Errorf("error[%v] is not expected[%v]", err, tc.wantErr)
			}
		}

		assertStacks(t, basedir, tc.want, stacks)
	}
}

func TestListMultipleSubStacks(t *testing.T) {
	n := 20
	stackdir := nSubStacks(t, n)

	defer removeStack(t, stackdir)

	stacks, err := terrastack.List(stackdir)
	assert.NoError(t, err, "terrastack.List")

	// n+1 because parent dir is also a stack
	assert.EqualInts(t, n+1, len(stacks), "stacks size mismatch")
}

func TestListChangedStacks(t *testing.T) {
	type testcase struct {
		repobuilder func(t *testing.T) string
		want        []string
		err         error
	}

	allrepos := []string{}

	defer func() {
		for _, repodir := range allrepos {
			os.RemoveAll(repodir)
		}
	}()

	for _, tc := range []testcase{
		{
			repobuilder: singleNotChangedStack,
			want:        []string{},
		},
		{
			repobuilder: singleNotChangedStackNewBranch,
			want:        []string{},
		},
		{
			repobuilder: singleNotMergeCommitBranch,
			want:        []string{"/"},
		},
		{
			repobuilder: singleChangedStacksRepo,
			want:        []string{"/"},
		},
		{
			repobuilder: multipleStacksOneChangedRepo,
			want:        []string{"/changed-stack"},
		},
		{
			repobuilder: multipleChangedStacksRepo,
			want: []string{
				"/changed-stack",
				"/changed-stack-0",
				"/changed-stack-1",
				"/changed-stack-2",
			},
		},
	} {
		repodir := tc.repobuilder(t)

		allrepos = append(allrepos, repodir)

		changed, err := terrastack.ListChanged(repodir)
		assert.EqualErrs(t, tc.err, err, "ListChanged() error")

		assert.EqualInts(t, len(tc.want), len(changed),
			"number of changed stacks mismatch")

		sort.Strings(changed)
		sort.Strings(tc.want)

		for i := 0; i < len(tc.want); i++ {
			assert.EqualStrings(t, filepath.Join(repodir, tc.want[i]),
				changed[i], "changed stack mismatch")
		}
	}
}

func assertStacks(t *testing.T, basedir string, want, got []string) {
	assert.EqualInts(t, len(want), len(got), "wrong number of stacks: %+v", got)

	for i := 0; i < len(want); i++ {
		index := strings.Index(got[i], basedir)
		assert.EqualInts(t, index, 0, "paths contains basedir")

		shifted := got[i][len(basedir):]
		if shifted == "" {
			shifted = "/"
		}
		assert.EqualStrings(t, want[i], shifted, "path mismatch")
	}
}

func singleStack(t *testing.T) string {
	stackdir := test.TempDir(t, "")

	err := terrastack.Init(stackdir, false)
	assert.NoError(t, err, "terrastack.Init(%q)", stackdir)

	return stackdir
}

func subStack(t *testing.T) string {
	stackdir := test.TempDir(t, "")

	err := terrastack.Init(stackdir, false)
	assert.NoError(t, err, "terrastack.Init(%q)", stackdir)

	substack := filepath.Join(stackdir, "substack")
	err = os.Mkdir(substack, 0644)
	assert.NoError(t, err, "creating substack")

	err = terrastack.Init(substack, false)
	assert.NoError(t, err, "terrastack.Init(%q)", substack)

	return stackdir
}

func nestedStacks(t *testing.T) string {
	stackdir := subStack(t)

	nestedStack := filepath.Join(stackdir, "substack", "deepstack")
	err := os.MkdirAll(nestedStack, 0644)
	assert.NoError(t, err, "creating nested stack dir")

	err = terrastack.Init(nestedStack, false)
	assert.NoError(t, err, "terrastack.Init(%q)", nestedStack)

	return stackdir
}

func nSubStacks(t *testing.T, n int) string {
	stackdir := test.TempDir(t, "")

	err := terrastack.Init(stackdir, false)
	assert.NoError(t, err, "terrastack.Init(%q)", stackdir)

	for i := 0; i < n; i++ {
		substack := test.TempDir(t, stackdir)

		err = terrastack.Init(substack, false)
		assert.NoError(t, err, "terrastack.Init(%q)", substack)
	}

	return stackdir
}

// singleChangedStacksRepo creates a new repository with the commands below:
//
// git init -b main <dir>
// cd <dir>
// terrastack init
// git add terrastack
// git commit -m "terrastack message"
// git checkout -b testbranch
// echo foo > foo
// git add foo
// git commit -m "foo message"
// git checkout main
// git merge testbranch
// git checkout -b testbranch2
// echo bar > bar
// git add bar
// git commit -m "bar message"
func singleChangedStacksRepo(t *testing.T) string {
	repodir := singleMergeCommitRepo(t)

	g := test.NewGitWrapper(t, repodir, false)

	assert.NoError(t, g.Checkout("testbranch2", true), "git checkout failed")

	test.CreateFile(t, repodir, "bar", "bar")

	assert.NoError(t, g.Add("bar"), "add bar failed")
	assert.NoError(t, g.Commit("bar message"), "bar commit failed")

	return repodir
}

// singleNotChangedStack returns a commited stack in main.
func singleNotChangedStack(t *testing.T) string {
	repodir := test.EmptyRepo(t)

	g := test.NewGitWrapper(t, repodir, false)

	// make it a stack
	assert.NoError(t, terrastack.Init(repodir, false), "terrastack init failed")
	assert.NoError(t, g.Add(terrastack.ConfigFilename), "add terrastack file failed")
	assert.NoError(t, g.Commit("terrastack message"), "terrastack commit failed")

	return repodir
}

// singleNotChangedStackNewBranch implements the behavior of returning "no
// changes" when the new branch revision matches the latest merge commit in
// main.
func singleNotChangedStackNewBranch(t *testing.T) string {
	repodir := singleNotChangedStack(t)

	g := test.NewGitWrapper(t, repodir, false)

	assert.NoError(t, g.Checkout("testbranch2", true), "git checkout failed")

	return repodir
}

func addMergeCommit(t *testing.T, repodir, branch string) {
	g := test.NewGitWrapper(t, repodir, false)

	assert.NoError(t, g.Checkout("main", false), "checkout main failed")
	assert.NoError(t, g.Merge(branch), "git merge failed")
}

func singleNotMergeCommitBranch(t *testing.T) string {
	repodir := singleNotChangedStack(t)

	g := test.NewGitWrapper(t, repodir, false)

	assert.NoError(t, g.Checkout("testbranch", true), "create branch failed")

	test.CreateFile(t, repodir, "foo", "foo")

	assert.NoError(t, g.Add("foo"), "add foo failed")
	assert.NoError(t, g.Commit("foo message"), "commit foo failed")

	return repodir
}

func singleMergeCommitRepo(t *testing.T) string {
	repodir := singleNotChangedStack(t)

	g := test.NewGitWrapper(t, repodir, false)

	assert.NoError(t, g.Checkout("testbranch", true), "create branch failed")

	test.CreateFile(t, repodir, "foo", "foo")

	assert.NoError(t, g.Add("foo"), "add foo failed")
	assert.NoError(t, g.Commit("foo message"), "commit foo failed")

	addMergeCommit(t, repodir, "testbranch")

	assert.NoError(t, g.DeleteBranch("testbranch"), "delete testbranch")

	return repodir
}

func multipleStacksOneChangedRepo(t *testing.T) string {
	repodir := singleMergeCommitRepo(t)

	g := test.NewGitWrapper(t, repodir, false)

	assert.NoError(t, g.Checkout("testbranch", true), "create branch failed")

	otherStack := filepath.Join(repodir, "not-changed-stack")
	assert.NoError(t, os.MkdirAll(otherStack, 0644), "mkdir otherstack failed")

	assert.NoError(t, terrastack.Init(otherStack, false), "terrastack init failed")

	assert.NoError(t, g.Add(filepath.Join(otherStack, terrastack.ConfigFilename)),
		"git add otherstack failed")
	assert.NoError(t, g.Commit("other stack message"), "commit failed")

	addMergeCommit(t, repodir, "testbranch")
	assert.NoError(t, g.DeleteBranch("testbranch"), "delete temp branch")

	// not merged changes
	assert.NoError(t, g.Checkout("testbranch2", true), "create branch testbranch2 failed")

	otherStack = filepath.Join(repodir, "changed-stack")
	assert.NoError(t, os.MkdirAll(otherStack, 0644), "mkdir otherstack failed")

	assert.NoError(t, terrastack.Init(otherStack, false), "terrastack init failed")

	assert.NoError(t, g.Add(filepath.Join(otherStack, terrastack.ConfigFilename)),
		"git add otherstack failed")
	assert.NoError(t, g.Commit("other stack message"), "commit failed")

	return repodir
}

func multipleChangedStacksRepo(t *testing.T) string {
	repodir := multipleStacksOneChangedRepo(t)

	g := test.NewGitWrapper(t, repodir, false)

	for i := 0; i < 3; i++ {
		otherStack := filepath.Join(repodir, "changed-stack-"+fmt.Sprint(i))
		assert.NoError(t, os.MkdirAll(otherStack, 0644), "mkdir otherstack failed")

		assert.NoError(t, terrastack.Init(otherStack, false), "terrastack init failed")

		assert.NoError(t, g.Add(filepath.Join(otherStack, terrastack.ConfigFilename)),
			"git add otherstack failed")
		assert.NoError(t, g.Commit("other stack message"), "commit failed")
	}

	return repodir
}
