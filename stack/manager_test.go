// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stack_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/stack"
	"github.com/terramate-io/terramate/test"
)

type repository struct {
	Dir     string
	modules []string
}

type listTestResult struct {
	list    []string
	changed []string
	err     error
}

type listTestcase struct {
	name        string
	baseRef     string
	repobuilder func(t *testing.T) repository
	want        listTestResult
}

const defaultBranch = "origin/main"

func TestListChangedStacks(t *testing.T) {
	for _, tc := range []listTestcase{
		{
			name:        "single stack: not changed",
			repobuilder: singleNotChangedStack,
			want: listTestResult{
				list: []string{"/"},
			},
		},
		{
			name:        "single stack: not changed but with empty module source",
			repobuilder: singleNotChangedStackWithEmptyModuleSrc,
			want: listTestResult{
				list: []string{"/"},
			},
		},
		{
			name:        "single stack: different base",
			repobuilder: singleNotChangedStack,
			baseRef:     "HEAD^",
			want: listTestResult{
				list:    []string{"/"},
				changed: []string{"/"},
			},
		},
		{
			name:        "single stack: not changed on a new branch",
			repobuilder: singleNotChangedStackNewBranch,
			want: listTestResult{
				list: []string{"/"},
			},
		},
		{
			name:        "single stack: not merged commit branch",
			repobuilder: singleNotMergedCommitBranch,
			want: listTestResult{
				list:    []string{"/"},
				changed: []string{"/"},
			},
		},
		{
			name:        "single stack: changed",
			repobuilder: singleChangedStacksRepo,
			want: listTestResult{
				list:    []string{"/"},
				changed: []string{"/"},
			},
		},
		{
			name:        "multiple stacks: one changed",
			repobuilder: multipleStacksOneChangedRepo,
			want: listTestResult{
				list:    []string{"/changed-stack", "/not-changed-stack"},
				changed: []string{"/changed-stack"},
			},
		},
		{
			name:        "multiple stacks: multiple changed",
			repobuilder: multipleChangedStacksRepo,
			want: listTestResult{
				list: []string{
					"/changed-stack",
					"/changed-stack-0",
					"/changed-stack-1",
					"/changed-stack-2",
					"/not-changed-stack",
				},
				changed: []string{
					"/changed-stack",
					"/changed-stack-0",
					"/changed-stack-1",
					"/changed-stack-2",
				},
			},
		},
		{
			name:        "single stack: single module changed",
			repobuilder: singleStackSingleModuleChangedRepo,
			want: listTestResult{
				list:    []string{"/stack"},
				changed: []string{"/stack"},
			},
		},
		{
			name:        "single stack: dependent module changed",
			repobuilder: singleStackDependentModuleChangedRepo,
			want: listTestResult{
				list:    []string{"/stack"},
				changed: []string{"/stack"},
			},
		},
		{
			name:        "multiple stack: single module changed",
			repobuilder: multipleStackOneChangedModule,
			want: listTestResult{
				list:    []string{"/stack1", "/stack2"},
				changed: []string{"/stack2"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.baseRef == "" {
				tc.baseRef = defaultBranch
			}

			repo := tc.repobuilder(t)
			root, err := config.LoadRoot(repo.Dir)
			assert.NoError(t, err)
			m := stack.NewManager(root, tc.baseRef)

			report, err := m.ListChanged()
			assert.EqualErrs(t, tc.want.err, err, "ListChanged() error")

			changedStacks := report.Stacks
			assertStacks(t, tc.want.changed, changedStacks, true)

			report, err = m.List()
			assert.EqualErrs(t, tc.want.err, err, "List() error")

			allstacks := report.Stacks
			assertStacks(t, tc.want.list, allstacks, false)
		})
	}
}

func TestListChangedStackReason(t *testing.T) {
	repo := singleNotMergedCommitBranch(t)

	m := newManager(t, repo.Dir)
	report, err := m.ListChanged()
	assert.NoError(t, err, "unexpected error")

	changed := report.Stacks
	assert.EqualInts(t, 1, len(changed), "unexpected number of entries")
	assert.EqualStrings(t, "/", changed[0].Stack.Dir.String(), "stack dir mismatch")
	assert.EqualStrings(t, "stack has unmerged changes", changed[0].Reason)

	repo = singleStackDependentModuleChangedRepo(t)

	m = newManager(t, repo.Dir)
	report, err = m.ListChanged()
	assert.NoError(t, err, "unexpected error")

	changed = report.Stacks
	assert.EqualInts(t, 1, len(changed), "unexpected number of entries")
	assert.EqualStrings(t, "/stack", changed[0].Stack.Dir.String(), "stack dir mismatch")

	if !strings.Contains(changed[0].Reason, "modules/module1") ||
		!strings.Contains(changed[0].Reason, "../module2") {
		t.Fatalf("unexpected reason %q (modules: %+v)", changed[0].Reason, repo.modules)
	}
}

func assertStacks(
	t *testing.T, want []string, got []stack.Entry, wantReason bool,
) {
	t.Helper()
	assert.EqualInts(t, len(want), len(got), "wrong number of stacks: %+v", got)

	for i := 0; i < len(want); i++ {
		assert.EqualStrings(t, want[i], got[i].Stack.Dir.String(), "path mismatch")

		if wantReason && got[i].Reason == "" {
			t.Errorf("stack [%s] has no reason", got[i].Stack.Dir)
		}
	}
}

// singleChangedStacksRepo creates a new repository with the commands below:
//
// git init -b main <dir>
// cd <dir>
// terramate init
// git add terramate
// git commit -m "terramate message"
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
func singleChangedStacksRepo(t *testing.T) repository {
	repo := singleMergeCommitRepo(t)

	g := test.NewGitWrapper(t, repo.Dir, []string{})

	assert.NoError(t, g.Checkout("testbranch2", true), "git checkout failed")

	_ = test.WriteFile(t, repo.Dir, "bar", "bar")

	assert.NoError(t, g.Add("bar"), "add bar failed")
	assert.NoError(t, g.Commit("bar message"), "bar commit failed")

	return repo
}

// singleNotChangedStack returns a committed stack in main.
func singleNotChangedStack(t *testing.T) repository {
	repo := test.NewRepo(t)

	g := test.NewGitWrapper(t, repo, []string{})

	root, err := config.LoadRoot(repo)
	assert.NoError(t, err)

	// make it a stack
	createStack(t, root, repo)
	assert.NoError(t, g.Add(stack.DefaultFilename), "add terramate file failed")
	assert.NoError(t, g.Commit("terramate message"), "terramate commit failed")

	// add a second commit to be able to test gitBaseRef=HEAD^
	readmePath := test.WriteFile(t, repo, "Something", "test")
	assert.NoError(t, g.Add(readmePath), "add terramate file failed")
	assert.NoError(t, g.Commit("add Something message"), "commit failed")

	assert.NoError(t, g.Push("origin", "main"), "push to origin")
	return repository{
		Dir: repo,
	}
}

func singleNotChangedStackWithEmptyModuleSrc(t *testing.T) repository {
	repo := singleNotChangedStack(t)
	g := test.NewGitWrapper(t, repo.Dir, []string{})
	test.WriteFile(t, repo.Dir, "main.tf", `
module "empty" {
	source = ""
}
`)

	assert.NoError(t, g.Add(repo.Dir), "add files")
	assert.NoError(t, g.Commit("files"), "commit files")
	assert.NoError(t, g.Push("origin", "main"), "push to origin")
	return repo
}

// singleNotChangedStackNewBranch implements the behavior of returning "no
// changes" when the new branch revision matches the latest merge commit in
// main.
func singleNotChangedStackNewBranch(t *testing.T) repository {
	repo := singleNotChangedStack(t)

	g := test.NewGitWrapper(t, repo.Dir, []string{})
	assert.NoError(t, g.Checkout("testbranch2", true), "git checkout failed")

	return repo
}

func addMergeCommit(t *testing.T, repodir, branch string) {
	g := test.NewGitWrapper(t, repodir, []string{})

	assert.NoError(t, g.Checkout("main", false), "checkout main failed")
	assert.NoError(t, g.Merge(branch), "git merge failed")
	assert.NoError(t, g.Push("origin", "main"), "git push origin main")
}

func singleNotMergedCommitBranch(t *testing.T) repository {
	repo := singleNotChangedStack(t)

	g := test.NewGitWrapper(t, repo.Dir, []string{})

	assert.NoError(t, g.Checkout("testbranch", true), "create branch failed")

	_ = test.WriteFile(t, repo.Dir, "foo", "foo")

	assert.NoError(t, g.Add("foo"), "add foo failed")
	assert.NoError(t, g.Commit("foo message"), "commit foo failed")

	return repo
}

func singleMergeCommitRepo(t *testing.T) repository {
	repo := singleNotChangedStack(t)

	g := test.NewGitWrapper(t, repo.Dir, []string{})

	assert.NoError(t, g.Checkout("testbranch", true), "create branch failed")

	_ = test.WriteFile(t, repo.Dir, "foo", "foo")

	assert.NoError(t, g.Add("foo"), "add foo failed")
	assert.NoError(t, g.Commit("foo message"), "commit foo failed")

	addMergeCommit(t, repo.Dir, "testbranch")

	assert.NoError(t, g.DeleteBranch("testbranch"), "delete testbranch")

	return repo
}

func singleMergeCommitRepoNoStack(t *testing.T) repository {
	repodir := test.NewRepo(t)
	repo := repository{
		Dir: repodir,
	}

	g := test.NewGitWrapper(t, repo.Dir, []string{})

	assert.NoError(t, g.Checkout("testbranch", true), "create branch failed")

	_ = test.WriteFile(t, repo.Dir, "foo", "foo")

	assert.NoError(t, g.Add("foo"), "add foo failed")
	assert.NoError(t, g.Commit("foo message"), "commit foo failed")

	addMergeCommit(t, repo.Dir, "testbranch")

	assert.NoError(t, g.DeleteBranch("testbranch"), "delete testbranch")

	return repo
}

func multipleStacksOneChangedRepo(t *testing.T) repository {
	repo := singleMergeCommitRepoNoStack(t)

	g := test.NewGitWrapper(t, repo.Dir, []string{})

	assert.NoError(t, g.Checkout("testbranch", true), "create branch failed")

	otherStack := filepath.Join(repo.Dir, "not-changed-stack")
	test.MkdirAll(t, otherStack)

	root, err := config.LoadRoot(repo.Dir)
	assert.NoError(t, err)
	createStack(t, root, otherStack)

	assert.NoError(t, g.Add(filepath.Join(otherStack, stack.DefaultFilename)),
		"git add otherstack failed")
	assert.NoError(t, g.Commit("other stack message"), "commit failed")

	addMergeCommit(t, repo.Dir, "testbranch")
	assert.NoError(t, g.DeleteBranch("testbranch"), "delete temp branch")

	// not merged changes
	assert.NoError(t, g.Checkout("testbranch2", true), "create branch testbranch2 failed")

	otherStack = filepath.Join(repo.Dir, "changed-stack")
	test.MkdirAll(t, otherStack)

	root, err = config.LoadRoot(repo.Dir)
	assert.NoError(t, err)
	createStack(t, root, otherStack)

	assert.NoError(t, g.Add(filepath.Join(otherStack, stack.DefaultFilename)),
		"git add otherstack failed")
	assert.NoError(t, g.Commit("other stack message"), "commit failed")
	return repo
}

func multipleChangedStacksRepo(t *testing.T) repository {
	repo := multipleStacksOneChangedRepo(t)

	g := test.NewGitWrapper(t, repo.Dir, []string{})

	root, err := config.LoadRoot(repo.Dir)
	assert.NoError(t, err)

	for i := 0; i < 3; i++ {
		otherStack := filepath.Join(repo.Dir, "changed-stack-"+fmt.Sprint(i))
		test.MkdirAll(t, otherStack)

		createStack(t, root, otherStack)
		assert.NoError(t, g.Add(filepath.Join(otherStack, stack.DefaultFilename)),
			"git add otherstack failed")
		assert.NoError(t, g.Commit("other stack message"), "commit failed")
	}

	return repo
}

func singleStackSingleModuleChangedRepo(t *testing.T) repository {
	repo := singleMergeCommitRepoNoStack(t)
	modules := test.Mkdir(t, repo.Dir, "modules")
	module1 := test.Mkdir(t, modules, "module1")
	module2 := test.Mkdir(t, modules, "module2")

	repo.modules = append(repo.modules, module1, module2)

	stack := test.Mkdir(t, repo.Dir, "stack")
	root, err := config.LoadRoot(repo.Dir)
	assert.NoError(t, err)
	createStack(t, root, stack)

	g := test.NewGitWrapper(t, repo.Dir, []string{})

	test.WriteFile(t, stack, "main.tf", `
module "something" {
	source = "../modules/module1"
}
`)

	assert.NoError(t, g.Add(repo.Dir), "add files")
	assert.NoError(t, g.Commit("files"), "commit files")

	return repo
}

func multipleStackOneChangedModule(t *testing.T) repository {
	repo := singleMergeCommitRepoNoStack(t)

	g := test.NewGitWrapper(t, repo.Dir, []string{})

	assert.NoError(t, g.Checkout("testbranch", true), "create branch failed")

	otherStack := filepath.Join(repo.Dir, "stack1")
	test.MkdirAll(t, otherStack)

	root, err := config.LoadRoot(repo.Dir)
	assert.NoError(t, err)
	createStack(t, root, otherStack)
	assert.NoError(t, g.Add(filepath.Join(otherStack, stack.DefaultFilename)),
		"git add otherstack failed")
	assert.NoError(t, g.Commit("other stack message"), "commit failed")

	otherStack = filepath.Join(repo.Dir, "stack2")
	test.MkdirAll(t, otherStack)

	assert.NoError(t, err)
	createStack(t, root, otherStack)
	assert.NoError(t, g.Add(filepath.Join(otherStack, stack.DefaultFilename)),
		"git add otherstack failed")
	assert.NoError(t, g.Commit("other stack message"), "commit failed")

	modules := test.Mkdir(t, repo.Dir, "modules")
	module := test.Mkdir(t, modules, "module1")

	mainFile := test.WriteFile(t, otherStack, "main.tf", `
module "something" {
	source = "../modules/module1"
}
`)

	assert.NoError(t, g.Add(mainFile), "add main.tf")
	assert.NoError(t, g.Commit("add main.tf"), "commit main.tf")

	addMergeCommit(t, repo.Dir, "testbranch")
	assert.NoError(t, g.DeleteBranch("testbranch"), "delete temp branch")

	mainFile = test.WriteFile(t, module, "main.tf", "")
	assert.NoError(t, g.Add(mainFile))
	assert.NoError(t, g.Commit("test"))
	assert.NoError(t, g.Push("origin", "main"), "push origin main")

	assert.NoError(t, g.Checkout("testbranch", true))
	mainFile = test.WriteFile(t, module, "main.tf", "# comment")
	assert.NoError(t, g.Add(mainFile))
	assert.NoError(t, g.Commit("test"))

	return repo
}

func singleStackDependentModuleChangedRepo(t *testing.T) repository {
	repo := singleMergeCommitRepoNoStack(t)

	modules := test.Mkdir(t, repo.Dir, "modules")
	module1 := test.Mkdir(t, modules, "module1")
	module2 := test.Mkdir(t, modules, "module2")

	repo.modules = append(repo.modules, module1, module2)

	stack := test.Mkdir(t, repo.Dir, "stack")
	root, err := config.LoadRoot(repo.Dir)
	assert.NoError(t, err)
	createStack(t, root, stack)
	g := test.NewGitWrapper(t, repo.Dir, []string{})

	test.WriteFile(t, stack, "main.tf", `
module "something" {
	source = "../modules/module1"
}
`)

	assert.NoError(t, g.Add(repo.Dir), "add files")
	assert.NoError(t, g.Commit("files"), "commit files")

	readmeFile := test.WriteFile(t, module2, "README.md", "GENERATED BY TERRAMATE TESTS!")
	assert.NoError(t, g.Add(readmeFile), "add readme file")
	assert.NoError(t, g.Commit("commit"), "commit readme")

	mainFile := test.WriteFile(t, module2, "main.tf", "")
	assert.NoError(t, g.Add(mainFile), "add main.tf")
	assert.NoError(t, g.Commit("commit main.tf"), "commit main.tf")

	mainFile = test.WriteFile(t, module1, "main.tf", `
module "module2" {
	source = "../module2"
}
`)

	assert.NoError(t, g.Add(mainFile), "add main.tf")
	assert.NoError(t, g.Commit("commit main.tf"), "commit main.tf")
	assert.NoError(t, g.Push("origin", "main"))

	assert.NoError(t, g.Checkout("change-module", true), "failed to create branch")
	mainFile = test.WriteFile(t, module2, "main.tf", `
# file changed
`)

	assert.NoError(t, g.Add(mainFile), "add main.tf")
	assert.NoError(t, g.Commit("commit main.tf"), "commit main.tf")

	return repo
}

func newManager(t *testing.T, basedir string) *stack.Manager {
	root, err := config.LoadRoot(basedir)
	assert.NoError(t, err)
	return stack.NewManager(root, defaultBranch)
}

func createStack(t *testing.T, root *config.Root, absdir string) {
	dir := project.PrjAbsPath(root.HostDir(), absdir)
	assert.NoError(t, stack.Create(root, config.Stack{Dir: dir}), "terramate init failed")
}
