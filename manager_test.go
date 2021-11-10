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

type listWant struct {
	list    []string
	changed []string
	err     error
}

type listTestcase struct {
	name        string
	repobuilder func(t *testing.T) (string, []string)
	want        listWant
}

func TestListStacks(t *testing.T) {
	for _, tc := range []listTestcase{
		{
			name:        "directory does not exists",
			repobuilder: nonExistentDir,
			want: listWant{
				err: os.ErrNotExist,
			},
		},
		{
			name:        "single stack",
			repobuilder: singleStack,
			want: listWant{
				list: []string{"/"},
			},
		},
		{
			name:        "stack and substack",
			repobuilder: subStack,
			want: listWant{
				list: []string{"/", "/substack"},
			},
		},
		{
			name:        "nested stacks",
			repobuilder: nestedStacks,
			want: listWant{
				list: []string{"/", "/substack", "/substack/deepstack"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, modules := tc.repobuilder(t)

			defer func() {
				assert.NoError(t, os.RemoveAll(repo), "failed to remove repo")

				for _, mod := range modules {
					assert.NoError(t, os.RemoveAll(mod), "failed to remove module dir")
				}
			}()

			m := terrastack.NewManager(repo)
			stacks, err := m.List()

			if tc.want.err != nil {
				if err == nil {
					t.Errorf("expected error: %v", tc.want.err)
				}

				if !errors.Is(err, tc.want.err) {
					t.Errorf("error[%v] is not expected[%v]", err, tc.want.err)
				}
			}

			sort.Strings(tc.want.list)
			assertStacks(t, repo, tc.want.list, stacks, false)
		})
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
	for _, tc := range []listTestcase{
		{
			name:        "single stack: not changed",
			repobuilder: singleNotChangedStack,
			want: listWant{
				list: []string{"/"},
			},
		},
		{
			name:        "single stack: not changed on a new branch",
			repobuilder: singleNotChangedStackNewBranch,
			want: listWant{
				list: []string{"/"},
			},
		},
		{
			name:        "single stack: not merged commit branch",
			repobuilder: singleNotMergedCommitBranch,
			want: listWant{
				list:    []string{"/"},
				changed: []string{"/"},
			},
		},
		{
			name:        "single stack: changed",
			repobuilder: singleChangedStacksRepo,
			want: listWant{
				list:    []string{"/"},
				changed: []string{"/"},
			},
		},
		{
			name:        "multiple stacks: one changed",
			repobuilder: multipleStacksOneChangedRepo,
			want: listWant{
				list:    []string{"/", "/changed-stack", "/not-changed-stack"},
				changed: []string{"/changed-stack"},
			},
		},
		{
			name:        "multiple stacks: multiple changed",
			repobuilder: multipleChangedStacksRepo,
			want: listWant{
				list: []string{
					"/",
					"/not-changed-stack",
					"/changed-stack",
					"/changed-stack-0",
					"/changed-stack-1",
					"/changed-stack-2",
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
			want: listWant{
				list:    []string{"/"},
				changed: []string{"/"},
			},
		},
		{
			name:        "single stack: dependent module changed",
			repobuilder: singleStackDependentModuleChangedRepo,
			want: listWant{
				list:    []string{"/"},
				changed: []string{"/"},
			},
		},
		{
			name:        "multiple stack: single module changed",
			repobuilder: multipleStackOneChangedModule,
			want: listWant{
				list:    []string{"/", "/stack1", "/stack2"},
				changed: []string{"/stack2"},
			},
		},
		{
			name:        "multiple stack: single module changed in same repo",
			repobuilder: multipleStackOneChangedModuleInSameRepo,
			want: listWant{
				list:    []string{"/", "/stack1", "/stack2"},
				changed: []string{"/stack2"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, modules := tc.repobuilder(t)

			defer func() {
				assert.NoError(t, os.RemoveAll(repo), "failed to remove repo")

				for _, mod := range modules {
					assert.NoError(t, os.RemoveAll(mod), "failed to remove module dir")
				}
			}()

			m := terrastack.NewManager(repo)

			changed, err := m.ListChanged()
			assert.EqualErrs(t, tc.want.err, err, "ListChanged() error")

			sort.Strings(tc.want.changed)
			assertStacks(t, repo, tc.want.changed, changed, true)

			list, err := m.List()
			assert.EqualErrs(t, tc.want.err, err, "List() error")
			sort.Strings(tc.want.list)
			assertStacks(t, repo, tc.want.list, list, false)
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

func nonExistentDir(t *testing.T) (string, []string) {
	return test.NonExistingDir(t), nil
}

func assertStacks(
	t *testing.T, basedir string, want []string, got []terrastack.Entry, hasReason bool,
) {
	assert.EqualInts(t, len(want), len(got), "wrong number of stacks: %+v", got)

	for i := 0; i < len(want); i++ {
		index := strings.Index(got[i].Dir, basedir)
		assert.EqualInts(t, index, 0, "paths contains basedir")

		shifted := got[i].Dir[len(basedir):]
		if shifted == "" {
			shifted = "/"
		}
		assert.EqualStrings(t, want[i], shifted, "path mismatch")

		if hasReason && got[i].Reason == "" {
			t.Errorf("stack [%s] has no reason", got[i].Dir)
		}
	}
}

func singleStack(t *testing.T) (string, []string) {
	stackdir := test.TempDir(t, "")

	mgr := terrastack.NewManager(stackdir)
	err := mgr.Init(stackdir, false)
	assert.NoError(t, err, "mgr.Init(%s)", stackdir)

	return stackdir, nil
}

func subStack(t *testing.T) (string, []string) {
	stackdir := test.TempDir(t, "")

	mgr := terrastack.NewManager(stackdir)
	err := mgr.Init(stackdir, false)
	assert.NoError(t, err, "mgr.Init(%s)", stackdir)

	substack := filepath.Join(stackdir, "substack")
	test.MkdirAll(t, substack)

	err = mgr.Init(substack, false)
	assert.NoError(t, err, "mgr.Init(%s)", substack)

	return stackdir, nil
}

func nestedStacks(t *testing.T) (string, []string) {
	stackdir, _ := subStack(t)

	nestedStack := filepath.Join(stackdir, "substack", "deepstack")
	test.MkdirAll(t, nestedStack)

	mgr := terrastack.NewManager(stackdir)
	err := mgr.Init(nestedStack, false)
	assert.NoError(t, err, "mgr.Init(%s)", nestedStack)

	return stackdir, nil
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

	_ = test.WriteFile(t, repo, "bar", "bar")

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

	_ = test.WriteFile(t, repo, "foo", "foo")

	assert.NoError(t, g.Add("foo"), "add foo failed")
	assert.NoError(t, g.Commit("foo message"), "commit foo failed")

	return repo, modules
}

func singleMergeCommitRepo(t *testing.T) (repo string, modules []string) {
	repo, modules = singleNotChangedStack(t)

	g := test.NewGitWrapper(t, repo, false)

	assert.NoError(t, g.Checkout("testbranch", true), "create branch failed")

	_ = test.WriteFile(t, repo, "foo", "foo")

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
	test.MkdirAll(t, otherStack)

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
	test.MkdirAll(t, otherStack)

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
		test.MkdirAll(t, otherStack)

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

	mainFile := test.WriteFile(t, repo, "main.tf", fmt.Sprintf(`
module "something" {
	source = "../../../../../..%s"
}
`, module))

	assert.NoError(t, g.Add(mainFile), "add main.tf")
	assert.NoError(t, g.Commit("add main.tf"), "commit main.tf")

	return repo, modules
}

func multipleStackOneChangedModule(t *testing.T) (repo string, modules []string) {
	repo, modules = singleMergeCommitRepo(t)

	g := test.NewGitWrapper(t, repo, false)

	assert.NoError(t, g.Checkout("testbranch", true), "create branch failed")

	otherStack := filepath.Join(repo, "stack1")
	test.MkdirAll(t, otherStack)

	mgr := terrastack.NewManager(repo)
	assert.NoError(t, mgr.Init(otherStack, false), "terrastack init failed")

	assert.NoError(t, g.Add(filepath.Join(otherStack, terrastack.ConfigFilename)),
		"git add otherstack failed")
	assert.NoError(t, g.Commit("other stack message"), "commit failed")

	otherStack = filepath.Join(repo, "stack2")
	test.MkdirAll(t, otherStack)

	assert.NoError(t, mgr.Init(otherStack, false), "terrastack init failed")

	assert.NoError(t, g.Add(filepath.Join(otherStack, terrastack.ConfigFilename)),
		"git add otherstack failed")
	assert.NoError(t, g.Commit("other stack message"), "commit failed")

	module := test.EmptyRepo(t)

	mainFile := test.WriteFile(t, otherStack, "main.tf", fmt.Sprintf(`
module "something" {
	source = "../../../../../../..%s"
}
`, module))

	assert.NoError(t, g.Add(mainFile), "add main.tf")
	assert.NoError(t, g.Commit("add main.tf"), "commit main.tf")

	addMergeCommit(t, repo, "testbranch")
	assert.NoError(t, g.DeleteBranch("testbranch"), "delete temp branch")

	g = test.NewGitWrapper(t, module, false)
	mainFile = test.WriteFile(t, module, "main.tf", "")
	assert.NoError(t, g.Add(mainFile))
	assert.NoError(t, g.Commit("test"))

	assert.NoError(t, g.Checkout("testbranch", true))
	mainFile = test.WriteFile(t, module, "main.tf", "# comment")
	assert.NoError(t, g.Add(mainFile))
	assert.NoError(t, g.Commit("test"))

	return repo, modules
}

func multipleStackOneChangedModuleInSameRepo(t *testing.T) (repo string, modules []string) {
	repo, modules = singleMergeCommitRepo(t)

	g := test.NewGitWrapper(t, repo, false)

	assert.NoError(t, g.Checkout("testbranch", true), "create branch failed")

	module := filepath.Join(repo, "modules/mymodule")
	test.MkdirAll(t, module)

	mainFile := test.WriteFile(t, module, "main.tf", "")
	assert.NoError(t, g.Add(mainFile))

	otherStack := filepath.Join(repo, "stack1")
	test.MkdirAll(t, otherStack)

	mgr := terrastack.NewManager(repo)
	assert.NoError(t, mgr.Init(otherStack, false), "terrastack init failed")

	assert.NoError(t, g.Add(filepath.Join(otherStack, terrastack.ConfigFilename)),
		"git add otherstack failed")
	assert.NoError(t, g.Commit("other stack message"), "commit failed")

	otherStack = filepath.Join(repo, "stack2")
	test.MkdirAll(t, otherStack)

	assert.NoError(t, mgr.Init(otherStack, false), "terrastack init failed")

	assert.NoError(t, g.Add(filepath.Join(otherStack, terrastack.ConfigFilename)),
		"git add otherstack failed")
	assert.NoError(t, g.Commit("other stack message"), "commit failed")

	mainFile = test.WriteFile(t, otherStack, "main.tf", fmt.Sprintf(`
module "something" {
	source = "../../../../../../..%s"
}
`, module))

	assert.NoError(t, g.Add(mainFile), "add main.tf")
	assert.NoError(t, g.Commit("add main.tf"), "commit main.tf")

	addMergeCommit(t, repo, "testbranch")
	assert.NoError(t, g.DeleteBranch("testbranch"), "delete temp branch")

	assert.NoError(t, g.Checkout("testbranch-module", true))
	mainFile = test.WriteFile(t, module, "main.tf", "# comment")
	assert.NoError(t, g.Add(mainFile))
	assert.NoError(t, g.Commit("test"))

	return repo, modules
}

func singleStackDependentModuleChangedRepo(t *testing.T) (repo string, modules []string) {
	repo, modules = singleNotChangedStack(t)
	module1, modules2 := singleNotChangedStack(t)

	modules = append(modules, module1)
	modules = append(modules, modules2...)

	g := test.NewGitWrapper(t, repo, false)

	mainFile := test.WriteFile(t, repo, "main.tf", fmt.Sprintf(`
module "module1" {
	source = "../../../../../..%s"
}
`, module1))

	assert.NoError(t, g.Add(mainFile), "add main.tf")
	assert.NoError(t, g.Commit("commit main.tf"), "commit main.tf")

	module2 := test.EmptyRepo(t)
	modules = append(modules, module2)

	g = test.NewGitWrapper(t, module2, false)

	readmeFile := test.WriteFile(t, module2, "README.md", "GENERATED BY TERRASTACK TESTS!")
	assert.NoError(t, g.Add(readmeFile), "add readme file")
	assert.NoError(t, g.Commit("commit"), "commit readme")
	assert.NoError(t, g.Checkout("add-module", true), "failed to create branch")

	mainFile = test.WriteFile(t, module2, "main.tf", "")
	assert.NoError(t, g.Add(mainFile), "add main.tf")
	assert.NoError(t, g.Commit("commit main.tf"), "commit main.tf")

	mainFile = test.WriteFile(t, module1, "main.tf", fmt.Sprintf(`
module "module2" {
	source = "../../../../../..%s"
}
`, module2))

	g = test.NewGitWrapper(t, module1, false)
	assert.NoError(t, g.Add(mainFile), "add main.tf")
	assert.NoError(t, g.Commit("commit main.tf"), "commit main.tf")

	return repo, modules
}
