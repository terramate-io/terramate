package terrastack_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terrastack"
	"github.com/mineiros-io/terrastack/test"
)

type repository struct {
	Dir        string
	OriginRepo string
	modules    []string
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

func TestListStacks(t *testing.T) {
	for _, tc := range []listTestcase{
		{
			name:        "directory does not exists",
			repobuilder: nonExistentDir,
			want: listTestResult{
				err: os.ErrNotExist,
			},
		},
		{
			name:        "single stack",
			repobuilder: singleStack,
			want: listTestResult{
				list: []string{"/"},
			},
		},
		{
			name:        "stack and substack",
			repobuilder: subStack,
			want: listTestResult{
				list: []string{"/", "/substack"},
			},
		},
		{
			name:        "nested stacks",
			repobuilder: nestedStacks,
			want: listTestResult{
				list: []string{"/", "/substack", "/substack/deepstack"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.baseRef == "" {
				tc.baseRef = defaultBranch
			}

			repo := tc.repobuilder(t)
			defer cleanupRepo(t, repo)

			m := terrastack.NewManager(repo.Dir, tc.baseRef)
			stacks, err := m.List()

			if !errors.Is(err, tc.want.err) {
				t.Fatalf("error[%v] is not expected[%v]", err, tc.want.err)
			}

			assertStacks(t, repo.Dir, tc.want.list, stacks, false)
		})
	}
}

func TestListMultipleSubStacks(t *testing.T) {
	n := 20
	stackdir := nSubStacks(t, n)

	defer removeStack(t, stackdir)

	m := newManager(stackdir)
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
				list:    []string{"/", "/changed-stack", "/not-changed-stack"},
				changed: []string{"/changed-stack"},
			},
		},
		{
			name:        "multiple stacks: multiple changed",
			repobuilder: multipleChangedStacksRepo,
			want: listTestResult{
				list: []string{
					"/",
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
				list:    []string{"/"},
				changed: []string{"/"},
			},
		},
		{
			name:        "single stack: dependent module changed",
			repobuilder: singleStackDependentModuleChangedRepo,
			want: listTestResult{
				list:    []string{"/"},
				changed: []string{"/"},
			},
		},
		{
			name:        "multiple stack: single module changed",
			repobuilder: multipleStackOneChangedModule,
			want: listTestResult{
				list:    []string{"/", "/stack1", "/stack2"},
				changed: []string{"/stack2"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.baseRef == "" {
				tc.baseRef = defaultBranch
			}

			repo := tc.repobuilder(t)
			defer cleanupRepo(t, repo)

			m := terrastack.NewManager(repo.Dir, tc.baseRef)

			changed, err := m.ListChanged()
			assert.EqualErrs(t, tc.want.err, err, "ListChanged() error")

			assertStacks(t, repo.Dir, tc.want.changed, changed, true)

			list, err := m.List()
			assert.EqualErrs(t, tc.want.err, err, "List() error")
			assertStacks(t, repo.Dir, tc.want.list, list, false)
		})
	}
}

func cleanupRepo(t *testing.T, repo repository) {
	test.RemoveAll(t, repo.Dir)

	if repo.OriginRepo != "" {
		test.RemoveAll(t, repo.OriginRepo)
	}

	for _, mod := range repo.modules {
		test.RemoveAll(t, mod)
	}
}

func TestListChangedStackReason(t *testing.T) {
	repo := singleNotMergedCommitBranch(t)
	defer cleanupRepo(t, repo)

	m := newManager(repo.Dir)
	changed, err := m.ListChanged()
	assert.NoError(t, err, "unexpected error")
	assert.EqualInts(t, 1, len(changed), "unexpected number of entries")
	assert.EqualStrings(t, repo.Dir, changed[0].Dir, "stack dir mismatch")
	assert.EqualStrings(t, "stack has unmerged changes", changed[0].Reason)

	repo = singleStackDependentModuleChangedRepo(t)
	defer cleanupRepo(t, repo)

	m = newManager(repo.Dir)
	changed, err = m.ListChanged()
	assert.NoError(t, err, "unexpected error")
	assert.EqualInts(t, 1, len(changed), "unexpected number of entries")
	assert.EqualStrings(t, repo.Dir, changed[0].Dir, "stack dir mismatch")

	if !strings.Contains(changed[0].Reason, "modules/module1") ||
		!strings.Contains(changed[0].Reason, "../module2") {
		t.Fatalf("unexpected reason %q (modules: %+v)", changed[0].Reason, repo.modules)
	}
}

func nonExistentDir(t *testing.T) repository {
	return repository{
		Dir: test.NonExistingDir(t),
	}
}

func assertStacks(
	t *testing.T, basedir string, want []string, got []terrastack.Entry, wantReason bool,
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

		if wantReason && got[i].Reason == "" {
			t.Errorf("stack [%s] has no reason", got[i].Dir)
		}
	}
}

func singleStack(t *testing.T) repository {
	stackdir := test.TempDir(t, "")

	mgr := newManager(stackdir)
	err := mgr.Init(stackdir, false)
	assert.NoError(t, err, "mgr.Init(%s)", stackdir)

	return repository{Dir: stackdir}
}

func subStack(t *testing.T) repository {
	stackdir := test.TempDir(t, "")

	mgr := newManager(stackdir)
	err := mgr.Init(stackdir, false)
	assert.NoError(t, err, "mgr.Init(%s)", stackdir)

	substack := filepath.Join(stackdir, "substack")
	test.MkdirAll(t, substack)

	err = mgr.Init(substack, false)
	assert.NoError(t, err, "mgr.Init(%s)", substack)

	return repository{Dir: stackdir}
}

func nestedStacks(t *testing.T) repository {
	stackrepo := subStack(t)

	nestedStack := filepath.Join(stackrepo.Dir, "substack", "deepstack")
	test.MkdirAll(t, nestedStack)

	mgr := newManager(stackrepo.Dir)
	err := mgr.Init(nestedStack, false)
	assert.NoError(t, err, "mgr.Init(%s)", nestedStack)

	return stackrepo
}

func nSubStacks(t *testing.T, n int) string {
	stackdir := test.TempDir(t, "")

	mgr := newManager(stackdir)
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
func singleChangedStacksRepo(t *testing.T) repository {
	repo := singleMergeCommitRepo(t)

	g := test.NewGitWrapper(t, repo.Dir, false)

	assert.NoError(t, g.Checkout("testbranch2", true), "git checkout failed")

	_ = test.WriteFile(t, repo.Dir, "bar", "bar")

	assert.NoError(t, g.Add("bar"), "add bar failed")
	assert.NoError(t, g.Commit("bar message"), "bar commit failed")

	return repo
}

// singleNotChangedStack returns a commited stack in main.
func singleNotChangedStack(t *testing.T) repository {
	repo, bare := test.TestRepo(t)

	g := test.NewGitWrapper(t, repo, false)

	// make it a stack
	mgr := newManager(repo)
	assert.NoError(t, mgr.Init(repo, false), "terrastack init failed")
	assert.NoError(t, g.Add(terrastack.ConfigFilename), "add terrastack file failed")
	assert.NoError(t, g.Commit("terrastack message"), "terrastack commit failed")

	// add a second commit to be able to test gitBaseRef=HEAD^
	readmePath := test.WriteFile(t, repo, "Something", "test")
	assert.NoError(t, g.Add(readmePath), "add terrastack file failed")
	assert.NoError(t, g.Commit("add Something message"), "commit failed")

	assert.NoError(t, g.Push("origin", "main"), "push to origin")
	return repository{
		Dir:        repo,
		OriginRepo: bare,
	}
}

// singleNotChangedStackNewBranch implements the behavior of returning "no
// changes" when the new branch revision matches the latest merge commit in
// main.
func singleNotChangedStackNewBranch(t *testing.T) repository {
	repo := singleNotChangedStack(t)

	g := test.NewGitWrapper(t, repo.Dir, false)
	assert.NoError(t, g.Checkout("testbranch2", true), "git checkout failed")

	return repo
}

func addMergeCommit(t *testing.T, repodir, branch string) {
	g := test.NewGitWrapper(t, repodir, false)

	assert.NoError(t, g.Checkout("main", false), "checkout main failed")
	assert.NoError(t, g.Merge(branch), "git merge failed")
	assert.NoError(t, g.Push("origin", "main"), "git push origin main")
}

func singleNotMergedCommitBranch(t *testing.T) repository {
	repo := singleNotChangedStack(t)

	g := test.NewGitWrapper(t, repo.Dir, false)

	assert.NoError(t, g.Checkout("testbranch", true), "create branch failed")

	_ = test.WriteFile(t, repo.Dir, "foo", "foo")

	assert.NoError(t, g.Add("foo"), "add foo failed")
	assert.NoError(t, g.Commit("foo message"), "commit foo failed")

	return repo
}

func singleMergeCommitRepo(t *testing.T) repository {
	repo := singleNotChangedStack(t)

	g := test.NewGitWrapper(t, repo.Dir, false)

	assert.NoError(t, g.Checkout("testbranch", true), "create branch failed")

	_ = test.WriteFile(t, repo.Dir, "foo", "foo")

	assert.NoError(t, g.Add("foo"), "add foo failed")
	assert.NoError(t, g.Commit("foo message"), "commit foo failed")

	addMergeCommit(t, repo.Dir, "testbranch")

	assert.NoError(t, g.DeleteBranch("testbranch"), "delete testbranch")

	return repo
}

func multipleStacksOneChangedRepo(t *testing.T) repository {
	repo := singleMergeCommitRepo(t)

	g := test.NewGitWrapper(t, repo.Dir, false)

	assert.NoError(t, g.Checkout("testbranch", true), "create branch failed")

	otherStack := filepath.Join(repo.Dir, "not-changed-stack")
	test.MkdirAll(t, otherStack)

	mgr := newManager(repo.Dir)
	assert.NoError(t, mgr.Init(otherStack, false), "terrastack init failed")

	assert.NoError(t, g.Add(filepath.Join(otherStack, terrastack.ConfigFilename)),
		"git add otherstack failed")
	assert.NoError(t, g.Commit("other stack message"), "commit failed")

	addMergeCommit(t, repo.Dir, "testbranch")
	assert.NoError(t, g.DeleteBranch("testbranch"), "delete temp branch")

	// not merged changes
	assert.NoError(t, g.Checkout("testbranch2", true), "create branch testbranch2 failed")

	otherStack = filepath.Join(repo.Dir, "changed-stack")
	test.MkdirAll(t, otherStack)

	assert.NoError(t, mgr.Init(otherStack, false), "terrastack init failed")

	assert.NoError(t, g.Add(filepath.Join(otherStack, terrastack.ConfigFilename)),
		"git add otherstack failed")
	assert.NoError(t, g.Commit("other stack message"), "commit failed")

	return repo
}

func multipleChangedStacksRepo(t *testing.T) repository {
	repo := multipleStacksOneChangedRepo(t)

	g := test.NewGitWrapper(t, repo.Dir, false)
	mgr := newManager(repo.Dir)

	for i := 0; i < 3; i++ {
		otherStack := filepath.Join(repo.Dir, "changed-stack-"+fmt.Sprint(i))
		test.MkdirAll(t, otherStack)

		assert.NoError(t, mgr.Init(otherStack, false), "terrastack init failed")

		assert.NoError(t, g.Add(filepath.Join(otherStack, terrastack.ConfigFilename)),
			"git add otherstack failed")
		assert.NoError(t, g.Commit("other stack message"), "commit failed")
	}

	return repo
}

func singleStackSingleModuleChangedRepo(t *testing.T) repository {
	repo := singleNotChangedStack(t)
	modules := test.Mkdir(t, repo.Dir, "modules")
	module1 := test.Mkdir(t, modules, "module1")
	module2 := test.Mkdir(t, modules, "module2")

	repo.modules = append(repo.modules, module1, module2)

	g := test.NewGitWrapper(t, repo.Dir, false)

	mainFile := test.WriteFile(t, repo.Dir, "main.tf", fmt.Sprintf(`
module "something" {
	source = "../../../../../..%s"
}
`, module1))

	assert.NoError(t, g.Add(mainFile), "add main.tf")
	assert.NoError(t, g.Commit("add main.tf"), "commit main.tf")

	return repo
}

func multipleStackOneChangedModule(t *testing.T) repository {
	repo := singleMergeCommitRepo(t)

	g := test.NewGitWrapper(t, repo.Dir, false)

	assert.NoError(t, g.Checkout("testbranch", true), "create branch failed")

	otherStack := filepath.Join(repo.Dir, "stack1")
	test.MkdirAll(t, otherStack)

	mgr := newManager(repo.Dir)
	assert.NoError(t, mgr.Init(otherStack, false), "terrastack init failed")

	assert.NoError(t, g.Add(filepath.Join(otherStack, terrastack.ConfigFilename)),
		"git add otherstack failed")
	assert.NoError(t, g.Commit("other stack message"), "commit failed")

	otherStack = filepath.Join(repo.Dir, "stack2")
	test.MkdirAll(t, otherStack)

	assert.NoError(t, mgr.Init(otherStack, false), "terrastack init failed")

	assert.NoError(t, g.Add(filepath.Join(otherStack, terrastack.ConfigFilename)),
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
	repo := singleNotChangedStack(t)

	modules := test.Mkdir(t, repo.Dir, "modules")
	module1 := test.Mkdir(t, modules, "module1")
	module2 := test.Mkdir(t, modules, "module2")

	repo.modules = append(repo.modules, module1, module2)

	g := test.NewGitWrapper(t, repo.Dir, false)

	mainFile := test.WriteFile(t, repo.Dir, "main.tf", `
module "module1" {
	source = "./modules/module1"
}
`)
	assert.NoError(t, g.Add(mainFile), "add main.tf")
	assert.NoError(t, g.Commit("commit main.tf"), "commit main.tf")

	readmeFile := test.WriteFile(t, module2, "README.md", "GENERATED BY TERRASTACK TESTS!")
	assert.NoError(t, g.Add(readmeFile), "add readme file")
	assert.NoError(t, g.Commit("commit"), "commit readme")

	mainFile = test.WriteFile(t, module2, "main.tf", "")
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

func newManager(basedir string) *terrastack.Manager {
	return terrastack.NewManager(basedir, defaultBranch)
}
