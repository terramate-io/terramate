package terrastack_test

import (
	"errors"
	"fmt"
	"io/ioutil"
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

		m := terrastack.NewManager(basedir)
		stacks, err := m.List()

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

	m := terrastack.NewManager(stackdir)

	stacks, err := m.List()
	assert.NoError(t, err, "terrastack.List")

	// n+1 because parent dir is also a stack
	assert.EqualInts(t, n+1, len(stacks), "stacks size mismatch")
}

func TestListChangedStacks(t *testing.T) {
	type testcase struct {
		name        string
		repobuilder func(t *testing.T) (string, []string)
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
			name:        "single stack: not changed",
			repobuilder: singleNotChangedStack,
			want:        []string{},
		},
		{
			name:        "single stack: not changed on a new branch",
			repobuilder: singleNotChangedStackNewBranch,
			want:        []string{},
		},
		{
			name:        "single stack: not merged commit branch",
			repobuilder: singleNotMergedCommitBranch,
			want:        []string{"/"},
		},
		{
			name:        "single stack: changed",
			repobuilder: singleChangedStacksRepo,
			want:        []string{"/"},
		},
		{
			name:        "multiple stacks: one changed",
			repobuilder: multipleStacksOneChangedRepo,
			want:        []string{"/changed-stack"},
		},
		{
			name:        "multiple stacks: multiple changed",
			repobuilder: multipleChangedStacksRepo,
			want: []string{
				"/changed-stack",
				"/changed-stack-0",
				"/changed-stack-1",
				"/changed-stack-2",
			},
		},
		{
			name:        "single stack: single module changed",
			repobuilder: singleStackSingleModuleChangedRepo,
			want:        []string{"/"},
		},
		{
			name:        "single stack: dependent module changed",
			repobuilder: singleStackDependentModuleChangedRepo,
			want:        []string{"/"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repodir, modules := tc.repobuilder(t)

			allrepos = append(allrepos, repodir)
			allrepos = append(allrepos, modules...)

			m := terrastack.NewManager(repodir)

			changed, err := m.ListChanged()
			assert.EqualErrs(t, tc.err, err, "ListChanged() error")

			assert.EqualInts(t, len(tc.want), len(changed),
				"number of changed stacks mismatch")

			sort.Sort(terrastack.EntrySlice(changed))
			sort.Strings(tc.want)

			for i := 0; i < len(tc.want); i++ {
				assert.EqualStrings(t, filepath.Join(repodir, tc.want[i]),
					changed[i].Dir, "changed stack mismatch")

				if changed[i].Reason == "" {
					t.Fatal("entry has no reason")
				}
			}
		})
	}
}

func TestListChangedStackReason(t *testing.T) {
	var removedirs []string

	repodir, modules := singleNotMergedCommitBranch(t)

	removedirs = append(removedirs, repodir)
	removedirs = append(removedirs, modules...)

	defer func() {
		for _, dir := range removedirs {
			os.RemoveAll(dir)
		}
	}()

	m := terrastack.NewManager(repodir)
	changed, err := m.ListChanged()
	assert.NoError(t, err, "unexpected error")
	assert.EqualInts(t, 1, len(changed), "unexpected number of entries")
	assert.EqualStrings(t, repodir, changed[0].Dir, "stack dir mismatch")
	assert.EqualStrings(t, "stack has unmerged changes", changed[0].Reason)

	repodir, modules = singleStackDependentModuleChangedRepo(t)
	removedirs = append(removedirs, repodir)
	removedirs = append(removedirs, modules...)

	m = terrastack.NewManager(repodir)
	changed, err = m.ListChanged()
	assert.NoError(t, err, "unexpected error")
	assert.EqualInts(t, 1, len(changed), "unexpected number of entries")
	assert.EqualStrings(t, repodir, changed[0].Dir, "stack dir mismatch")

	if !strings.Contains(changed[0].Reason, modules[0]) ||
		!strings.Contains(changed[0].Reason, modules[1]) {
		t.Fatalf("unexpected reason %q", changed[0].Reason)

	}
}

func assertStacks(t *testing.T, basedir string, want []string, got []terrastack.Entry) {
	assert.EqualInts(t, len(want), len(got), "wrong number of stacks: %+v", got)

	for i := 0; i < len(want); i++ {
		index := strings.Index(got[i].Dir, basedir)
		assert.EqualInts(t, index, 0, "paths contains basedir")

		shifted := got[i].Dir[len(basedir):]
		if shifted == "" {
			shifted = "/"
		}
		assert.EqualStrings(t, want[i], shifted, "path mismatch")
	}
}

func singleStack(t *testing.T) string {
	stackdir := test.TempDir(t, "")

	mgr := terrastack.NewManager(stackdir)
	err := mgr.Init(stackdir, false)
	assert.NoError(t, err, "mgr.Init(%s)", stackdir)

	return stackdir
}

func subStack(t *testing.T) string {
	stackdir := test.TempDir(t, "")

	mgr := terrastack.NewManager(stackdir)
	err := mgr.Init(stackdir, false)
	assert.NoError(t, err, "mgr.Init(%s)", stackdir)

	substack := filepath.Join(stackdir, "substack")
	err = os.Mkdir(substack, 0700)
	assert.NoError(t, err, "creating substack")

	err = mgr.Init(substack, false)
	assert.NoError(t, err, "mgr.Init(%s)", substack)

	return stackdir
}

func nestedStacks(t *testing.T) string {
	stackdir := subStack(t)

	nestedStack := filepath.Join(stackdir, "substack", "deepstack")
	err := os.MkdirAll(nestedStack, 0700)
	assert.NoError(t, err, "creating nested stack dir")

	mgr := terrastack.NewManager(stackdir)
	err = mgr.Init(nestedStack, false)
	assert.NoError(t, err, "mgr.Init(%s)", nestedStack)

	return stackdir
}

func nSubStacks(t *testing.T, n int) string {
	stackdir := test.TempDir(t, "")

	mgr := terrastack.NewManager(stackdir)
	err := mgr.Init(stackdir, false)
	assert.NoError(t, err, "mgr.Init(%s)", stackdir)

	for i := 0; i < n; i++ {
		substack := test.TempDir(t, stackdir)

		err = mgr.Init(substack, false)
		assert.NoError(t, err, "mgr.Init(%s)", substack)
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
func singleChangedStacksRepo(t *testing.T) (repo string, modules []string) {
	repo, modules = singleMergeCommitRepo(t)

	g := test.NewGitWrapper(t, repo, false)

	assert.NoError(t, g.Checkout("testbranch2", true), "git checkout failed")

	test.CreateFile(t, repo, "bar", "bar")

	assert.NoError(t, g.Add("bar"), "add bar failed")
	assert.NoError(t, g.Commit("bar message"), "bar commit failed")

	return repo, modules
}

// singleNotChangedStack returns a commited stack in main.
func singleNotChangedStack(t *testing.T) (repo string, modules []string) {
	repo = test.EmptyRepo(t)

	g := test.NewGitWrapper(t, repo, false)

	// make it a stack
	mgr := terrastack.NewManager(repo)
	assert.NoError(t, mgr.Init(repo, false), "terrastack init failed")
	assert.NoError(t, g.Add(terrastack.ConfigFilename), "add terrastack file failed")
	assert.NoError(t, g.Commit("terrastack message"), "terrastack commit failed")

	return repo, nil
}

// singleNotChangedStackNewBranch implements the behavior of returning "no
// changes" when the new branch revision matches the latest merge commit in
// main.
func singleNotChangedStackNewBranch(t *testing.T) (repo string, modules []string) {
	repo, modules = singleNotChangedStack(t)

	g := test.NewGitWrapper(t, repo, false)

	assert.NoError(t, g.Checkout("testbranch2", true), "git checkout failed")

	return repo, modules
}

func addMergeCommit(t *testing.T, repodir, branch string) {
	g := test.NewGitWrapper(t, repodir, false)

	assert.NoError(t, g.Checkout("main", false), "checkout main failed")
	assert.NoError(t, g.Merge(branch), "git merge failed")
}

func singleNotMergedCommitBranch(t *testing.T) (repo string, modules []string) {
	repo, modules = singleNotChangedStack(t)

	g := test.NewGitWrapper(t, repo, false)

	assert.NoError(t, g.Checkout("testbranch", true), "create branch failed")

	test.CreateFile(t, repo, "foo", "foo")

	assert.NoError(t, g.Add("foo"), "add foo failed")
	assert.NoError(t, g.Commit("foo message"), "commit foo failed")

	return repo, modules
}

func singleMergeCommitRepo(t *testing.T) (repo string, modules []string) {
	repo, modules = singleNotChangedStack(t)

	g := test.NewGitWrapper(t, repo, false)

	assert.NoError(t, g.Checkout("testbranch", true), "create branch failed")

	test.CreateFile(t, repo, "foo", "foo")

	assert.NoError(t, g.Add("foo"), "add foo failed")
	assert.NoError(t, g.Commit("foo message"), "commit foo failed")

	addMergeCommit(t, repo, "testbranch")

	assert.NoError(t, g.DeleteBranch("testbranch"), "delete testbranch")

	return repo, modules
}

func multipleStacksOneChangedRepo(t *testing.T) (repo string, modules []string) {
	repo, modules = singleMergeCommitRepo(t)

	g := test.NewGitWrapper(t, repo, false)

	assert.NoError(t, g.Checkout("testbranch", true), "create branch failed")

	otherStack := filepath.Join(repo, "not-changed-stack")
	assert.NoError(t, os.MkdirAll(otherStack, 0700), "mkdir otherstack failed")

	mgr := terrastack.NewManager(repo)
	assert.NoError(t, mgr.Init(otherStack, false), "terrastack init failed")

	assert.NoError(t, g.Add(filepath.Join(otherStack, terrastack.ConfigFilename)),
		"git add otherstack failed")
	assert.NoError(t, g.Commit("other stack message"), "commit failed")

	addMergeCommit(t, repo, "testbranch")
	assert.NoError(t, g.DeleteBranch("testbranch"), "delete temp branch")

	// not merged changes
	assert.NoError(t, g.Checkout("testbranch2", true), "create branch testbranch2 failed")

	otherStack = filepath.Join(repo, "changed-stack")
	assert.NoError(t, os.MkdirAll(otherStack, 0700), "mkdir otherstack failed")

	assert.NoError(t, mgr.Init(otherStack, false), "terrastack init failed")

	assert.NoError(t, g.Add(filepath.Join(otherStack, terrastack.ConfigFilename)),
		"git add otherstack failed")
	assert.NoError(t, g.Commit("other stack message"), "commit failed")

	return repo, modules
}

func multipleChangedStacksRepo(t *testing.T) (repo string, modules []string) {
	repo, modules = multipleStacksOneChangedRepo(t)

	g := test.NewGitWrapper(t, repo, false)
	mgr := terrastack.NewManager(repo)

	for i := 0; i < 3; i++ {
		otherStack := filepath.Join(repo, "changed-stack-"+fmt.Sprint(i))
		assert.NoError(t, os.MkdirAll(otherStack, 0700), "mkdir otherstack failed")

		assert.NoError(t, mgr.Init(otherStack, false), "terrastack init failed")

		assert.NoError(t, g.Add(filepath.Join(otherStack, terrastack.ConfigFilename)),
			"git add otherstack failed")
		assert.NoError(t, g.Commit("other stack message"), "commit failed")
	}

	return repo, modules
}

func singleStackSingleModuleChangedRepo(t *testing.T) (repo string, modules []string) {
	repo, modules = singleNotChangedStack(t)
	module, modules2 := singleChangedStacksRepo(t)

	modules = append(modules, module)
	modules = append(modules, modules2...)

	g := test.NewGitWrapper(t, repo, false)

	mainFile := filepath.Join(repo, "main.tf")
	err := os.WriteFile(mainFile, []byte(fmt.Sprintf(`
module "something" {
	source = "../../../../../..%s"
}
`, module)), 0700)

	assert.NoError(t, err, "write main.tf")

	assert.NoError(t, g.Add(mainFile), "add main.tf")
	assert.NoError(t, g.Commit("add main.tf"), "commit main.tf")

	return repo, modules
}

func singleStackDependentModuleChangedRepo(t *testing.T) (repo string, modules []string) {
	repo, modules = singleNotChangedStack(t)
	module1, modules2 := singleNotChangedStack(t)

	modules = append(modules, module1)
	modules = append(modules, modules2...)

	g := test.NewGitWrapper(t, repo, false)

	mainFile := filepath.Join(repo, "main.tf")
	err := os.WriteFile(mainFile, []byte(fmt.Sprintf(`
module "module1" {
	source = "../../../../../..%s"
}
`, module1)), 0700)

	assert.NoError(t, err, "write main.tf")
	assert.NoError(t, g.Add(mainFile), "add main.tf")
	assert.NoError(t, g.Commit("commit main.tf"), "commit main.tf")

	module2 := test.EmptyRepo(t)
	modules = append(modules, module2)

	g = test.NewGitWrapper(t, module2, false)

	readmeFile := filepath.Join(module2, "README.md")
	err = ioutil.WriteFile(readmeFile, []byte(
		"GENERATED BY TERRASTACK TESTS!",
	), 0700)
	assert.NoError(t, err, "writing README file")

	assert.NoError(t, g.Add(readmeFile), "add readme file")
	assert.NoError(t, g.Commit("commit"), "commit readme")
	assert.NoError(t, g.Checkout("add-module", true), "failed to create branch")

	mainFile = filepath.Join(module2, "main.tf")
	err = ioutil.WriteFile(mainFile, []byte(""), 0700)
	assert.NoError(t, err, "creating empty main.tf")

	assert.NoError(t, g.Add(mainFile), "add main.tf")
	assert.NoError(t, g.Commit("commit main.tf"), "commit main.tf")

	mainFile = filepath.Join(module1, "main.tf")
	err = os.WriteFile(mainFile, []byte(fmt.Sprintf(`
module "module2" {
	source = "../../../../../..%s"
}
`, module2)), 0700)

	assert.NoError(t, err, "write main.tf")
	g = test.NewGitWrapper(t, module1, false)
	assert.NoError(t, g.Add(mainFile), "add main.tf")
	assert.NoError(t, g.Commit("commit main.tf"), "commit main.tf")

	return repo, modules
}
