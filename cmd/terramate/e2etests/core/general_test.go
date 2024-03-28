// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cmd/terramate/cli"
	. "github.com/terramate-io/terramate/cmd/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestBug25(t *testing.T) {
	t.Parallel()

	// bug: https://github.com/terramate-io/terramate/issues/25

	const (
		modname1 = "1"
		modname2 = "2"
	)

	s := sandbox.New(t)

	mod1 := s.CreateModule(modname1)
	mod1MainTf := mod1.CreateFile("main.tf", "# module 1")

	mod2 := s.CreateModule(modname2)
	mod2.CreateFile("main.tf", "# module 2")

	stack1 := s.CreateStack("stack-1")
	stack2 := s.CreateStack("stack-2")
	stack3 := s.CreateStack("stack-3")

	stack1.CreateFile("main.tf", `
module "mod1" {
source = "%s"
}`, stack1.ModSource(mod1))

	stack2.CreateFile("main.tf", `
module "mod2" {
source = "%s"
}`, stack2.ModSource(mod2))

	stack3.CreateFile("main.tf", "# no module")

	git := s.Git()
	git.CommitAll("first commit")
	git.Push("main")
	git.CheckoutNew("change-the-module-1")

	mod1MainTf.Write("# changed")

	git.CommitAll("module 1 changed")

	cli := NewCLI(t, s.RootDir())
	want := stack1.RelPath() + "\n"
	AssertRunResult(t, cli.ListChangedStacks(), RunExpected{Stdout: want})
}

func TestBugModuleMultipleFilesSameDir(t *testing.T) {
	t.Parallel()

	const (
		modname1 = "1"
		modname2 = "2"
		modname3 = "3"
	)

	s := sandbox.New(t)

	mod2 := s.CreateModule(modname2)
	mod2MainTf := mod2.CreateFile("main.tf", "# module 2")

	mod3 := s.CreateModule(modname2)
	mod3.CreateFile("main.tf", "# module 3")

	// This issue is related to multiple files in the module directory and the
	// order of the changed one is important, it should come first, with other
	// files with module declarations skipped (module source not local).
	// The files are named "1.tf" and "2.tf" because filepath.Walk() does a
	// lexicographic walking of the files.
	mod1 := s.CreateModule(modname1)
	mod1.CreateFile("1.tf", `
module "changed" {
	source = "../2"
}
	`)

	mod1.CreateFile("2.tf", `
module "any" {
	source = "anything"
}

module "any2" {
	source = "anything"
}
`)

	stack := s.CreateStack("stack")

	stack.CreateFile("main.tf", `
module "mod1" {
    source = %q
}
`, stack.ModSource(mod1))

	git := s.Git()
	git.CommitAll("first commit")
	git.Push("main")
	git.CheckoutNew("change-the-module-2")

	mod2MainTf.Write("# changed")

	git.CommitAll("module 2 changed")

	cli := NewCLI(t, s.RootDir())
	want := stack.RelPath() + "\n"
	AssertRunResult(t, cli.ListChangedStacks(), RunExpected{Stdout: want})
}

func TestListAndRunChangedStack(t *testing.T) {
	t.Parallel()

	const (
		mainTfFileName = "main.tf"
		mainTfContents = "# change is the eternal truth of the universe"
	)

	s := sandbox.New(t)

	stack := s.CreateStack("stack")
	stackMainTf := stack.CreateFile(mainTfFileName, "# some code")

	cli := NewCLI(t, s.RootDir())

	git := s.Git()
	git.CommitAll("first commit")
	git.Push("main")
	git.CheckoutNew("change-stack")

	stackMainTf.Write(mainTfContents)
	git.CommitAll("stack changed")

	wantList := stack.RelPath() + "\n"
	AssertRunResult(t, cli.ListChangedStacks(), RunExpected{Stdout: wantList})

	wantRun := mainTfContents

	AssertRunResult(t, cli.Run(
		"run",
		"--quiet",
		"--changed",
		HelperPath,
		"cat",
		mainTfFileName,
	), RunExpected{
		Stdout: wantRun,
	})
}

func TestListAndRunChangedStackInAbsolutePath(t *testing.T) {
	t.SkipNow()
	const (
		mainTfFileName = "main.tf"
		mainTfContents = "# change is the eternal truth of the universe"
	)

	s := sandbox.New(t)

	stack := s.CreateStack("stack")
	stackMainTf := stack.CreateFile(mainTfFileName, "# some code")

	cli := NewCLI(t, test.TempDir(t))

	git := s.Git()
	git.CommitAll("first commit")
	git.Push("main")
	git.CheckoutNew("change-stack")

	stackMainTf.Write(mainTfContents)
	git.CommitAll("stack changed")

	wantList := stack.Path() + "\n"
	AssertRunResult(t, cli.ListChangedStacks(), RunExpected{Stdout: wantList})

	wantRun := fmt.Sprintf(
		"Running on changed stacks:\n[%s] running %s %s %s\n%s\n",
		stack.Path(),
		HelperPath,
		"cat",
		mainTfFileName,
		mainTfContents,
	)

	AssertRunResult(t, cli.Run(
		"run",
		"--quiet",
		"--changed",
		HelperPath,
		"cat",
		mainTfFileName,
	), RunExpected{Stdout: wantRun})
}

func TestDefaultBaseRefInOtherThanMain(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)

	stack := s.CreateStack("stack-1")
	stackFile := stack.CreateFile("main.tf", "# no code")

	cli := NewCLI(t, s.RootDir())

	git := s.Git()
	git.Add(".")
	git.Commit("all")
	git.Push("main")
	git.CheckoutNew("change-the-stack")

	stackFile.Write("# changed")
	git.Add(stack.Path())
	git.Commit("stack changed")

	want := RunExpected{
		Stdout: stack.RelPath() + "\n",
	}
	AssertRunResult(t, cli.ListChangedStacks(), want)
}

func TestDefaultBaseRefInMain(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)

	stack := s.CreateStack("stack-1")
	stack.CreateFile("main.tf", "# no code")

	cli := NewCLI(t, s.RootDir())

	git := s.Git()
	git.Add(".")
	git.Commit("all")
	git.Push("main")

	// main uses HEAD^ as default baseRef.
	want := RunExpected{
		Stdout: stack.RelPath() + "\n",
	}
	AssertRunResult(t, cli.ListChangedStacks(), want)
}

func TestChangeBaseSelection1(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	cli := NewCLI(t, s.RootDir())
	git := s.Git()

	hashToName := map[string]string{"": ""}
	nameToHash := map[string]string{"": ""}

	setNamedCommit := func(name string) {
		hash := git.RevParse("HEAD")

		hashToName[hash] = name
		nameToHash[name] = hash
	}

	makeStackCommit := func(name string) {
		st := s.CreateStack(name)
		st.CreateFile("main.tf", "# none")
		git.Add(name)
		git.Commit(name)

		setNamedCommit(name)
	}

	type testcase struct {
		Commit string
		Ref    string

		WantChanged []string
	}

	var tests []testcase

	makeStackCommit("main_c1")

	tests = append(tests, []testcase{
		{
			Commit: "main_c1",
			WantChanged: []string{
				"main_c1",
			},
		}}...,
	)

	git.CheckoutNew("merged1")
	makeStackCommit("merged1_c1")
	makeStackCommit("merged1_c2")

	tests = append(tests, []testcase{
		{
			Commit: "merged1_c1",
			WantChanged: []string{
				"merged1_c1",
			},
		},
		{
			Commit: "merged1_c2",
			WantChanged: []string{
				"merged1_c1",
				"merged1_c2",
			},
		},
		{
			Ref: "merged1",
			WantChanged: []string{
				"merged1_c1",
				"merged1_c2",
			},
		}}...,
	)

	git.Checkout("main")
	git.Merge("merged1")
	setNamedCommit("main_c2")

	tests = append(tests, []testcase{
		{
			Commit: "main_c2",
			WantChanged: []string{
				"merged1_c1",
				"merged1_c2",
			},
		}}...,
	)

	git.CheckoutNew("unmerged")
	makeStackCommit("unmerged_c1")

	tests = append(tests, []testcase{
		{
			Commit: "unmerged_c1",
			WantChanged: []string{
				"unmerged_c1",
			},
		},
		{
			Ref: "unmerged",
			WantChanged: []string{
				"unmerged_c1",
			},
		}}...,
	)

	git.Checkout("main")

	git.CheckoutNew("merged2")
	makeStackCommit("merged2_c1")

	tests = append(tests, []testcase{
		{
			Commit: "merged2_c1",
			WantChanged: []string{
				"merged2_c1",
			},
		},
		{
			Ref: "merged2",
			WantChanged: []string{
				"merged2_c1",
			},
		}}...,
	)

	git.Checkout("main")
	git.Merge("merged2")
	setNamedCommit("main_c3")

	git.Push("main") // origin/main -> main_c3

	tests = append(tests, []testcase{
		{
			Commit: "main_c3",
			WantChanged: []string{
				"merged2_c1",
			},
		},
		{
			Ref: "origin/main",
			WantChanged: []string{
				"merged2_c1",
			},
		}}...,
	)

	git.CheckoutNew("empty")

	tests = append(tests, []testcase{
		{
			Ref:         "empty",
			WantChanged: []string{},
		}}...,
	)

	git.CheckoutNew("wip")
	makeStackCommit("wip_c1")
	makeStackCommit("wip_c2")

	tests = append(tests, []testcase{
		{
			Commit: "wip_c1",
			WantChanged: []string{
				"wip_c1",
			},
		},
		{
			Commit: "wip_c2",
			WantChanged: []string{
				"wip_c1",
				"wip_c2",
			},
		},
		{
			Ref: "wip",
			WantChanged: []string{
				"wip_c1",
				"wip_c2",
			},
		}}...,
	)

	git.Checkout("main")
	makeStackCommit("main_c4")
	makeStackCommit("main_c5")

	tests = append(tests, []testcase{
		{
			Commit: "main_c4",
			WantChanged: []string{
				"main_c4",
			},
		},
		{
			Commit: "main_c5",
			WantChanged: []string{
				"main_c4",
				"main_c5",
			},
		},
		{
			Ref: "main",
			WantChanged: []string{
				"main_c5",
			},
		}}...,
	)

	for _, tc := range tests {
		var name string
		if tc.Commit != "" {
			name = tc.Commit
		} else {
			name = tc.Ref
		}

		t.Run(name, func(t *testing.T) {
			assert.IsTrue(t, (tc.Commit != "") != (tc.Ref != ""), "set either commit or ref")
			var rev string
			if tc.Commit != "" {
				rev = nameToHash[tc.Commit]
			} else {
				rev = tc.Ref
			}

			git.Checkout(rev)

			wantStdout := ""
			if len(tc.WantChanged) != 0 {
				wantStdout = strings.Join(tc.WantChanged, "\n") + "\n"
			}

			want := RunExpected{Stdout: wantStdout}
			AssertRunResult(t, cli.ListChangedStacks(), want)
		})
	}
}

func TestChangeBaseSelection2(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	cli := NewCLI(t, s.RootDir())
	git := s.Git()

	hashToName := map[string]string{"": ""}
	nameToHash := map[string]string{"": ""}

	setNamedCommit := func(name string) {
		hash := git.RevParse("HEAD")

		hashToName[hash] = name
		nameToHash[name] = hash
	}

	makeStackCommit := func(name string) {
		st := s.CreateStack(name)
		st.CreateFile("main.tf", "# none")
		git.Add(name)
		git.Commit(name)

		setNamedCommit(name)
	}

	type testcase struct {
		Commit string
		Ref    string

		WantChanged []string
	}

	var tests []testcase

	makeStackCommit("A")

	tests = append(tests, []testcase{
		{
			Commit: "A",
			WantChanged: []string{
				"A",
			},
		}}...,
	)

	git.CheckoutNew("merged")
	makeStackCommit("A0")
	git.CheckoutNew("unmerged")
	makeStackCommit("A2")
	git.Checkout("merged")
	makeStackCommit("A1")

	tests = append(tests, []testcase{
		{
			Commit: "A0",
			WantChanged: []string{
				"A0",
			},
		},
		{
			Commit: "A1",
			WantChanged: []string{
				"A0",
				"A1",
			},
		},
		{
			Ref: "merged",
			WantChanged: []string{
				"A0",
				"A1",
			},
		},
		{
			Commit: "A2",
			WantChanged: []string{
				"A0",
				"A2",
			},
		},
		{
			Ref: "unmerged",
			WantChanged: []string{
				"A0",
				"A2",
			},
		}}...,
	)

	git.Checkout("main")
	git.Merge("merged")
	setNamedCommit("B")

	git.CheckoutNew("empty")

	tests = append(tests, []testcase{
		{
			Ref:         "empty",
			WantChanged: []string{},
		}}...,
	)

	git.Checkout("main")
	git.CheckoutNew("wip")
	makeStackCommit("B0")

	git.Checkout("main")
	makeStackCommit("C")

	git.Checkout("wip")
	git.Merge("main")
	setNamedCommit("C0")

	git.Checkout("main")
	makeStackCommit("D")

	tests = append(tests, []testcase{
		{
			Commit: "B0",
			WantChanged: []string{
				"B0",
			},
		},
		{
			Commit: "C",
			WantChanged: []string{
				"C",
			},
		},
		{
			Commit: "C0",
			WantChanged: []string{
				"B0",
			},
		},
		{
			Ref: "wip",
			WantChanged: []string{
				"B0",
			},
		},
		{
			Commit: "D",
			WantChanged: []string{
				"D",
			},
		},
		{
			Ref: "main",
			WantChanged: []string{
				"D",
			},
		}}...,
	)

	git.Push("main")

	for _, tc := range tests {
		var name string
		if tc.Commit != "" {
			name = tc.Commit
		} else {
			name = tc.Ref
		}

		t.Run(name, func(t *testing.T) {
			assert.IsTrue(t, (tc.Commit != "") != (tc.Ref != ""), "set either commit or ref")
			var rev string
			if tc.Commit != "" {
				rev = nameToHash[tc.Commit]
			} else {
				rev = tc.Ref
			}

			git.Checkout(rev)

			wantStdout := ""
			if len(tc.WantChanged) != 0 {
				wantStdout = strings.Join(tc.WantChanged, "\n") + "\n"
			}

			want := RunExpected{Stdout: wantStdout}
			AssertRunResult(t, cli.ListChangedStacks(), want)
		})
	}
}

func TestBaseRefFlagPrecedenceOverDefault(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)

	stack := s.CreateStack("stack-1")
	stack.CreateFile("main.tf", "# no code")

	cli := NewCLI(t, s.RootDir())

	git := s.Git()
	git.Add(".")
	git.Commit("all")
	git.Push("main")

	AssertRunResult(t, cli.ListChangedStacks("--git-change-base", "origin/main"),
		RunExpected{},
	)
}

func TestMainAfterOriginMainMustUseDefaultBaseRef(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	ts := NewCLI(t, s.RootDir())

	createCommittedStack := func(name string) {
		stack := s.CreateStack(name)
		stack.CreateFile("main.tf", "# no code")

		git := s.Git()
		git.Add(".")
		git.Commit(name)
	}

	// creates N commits in main.
	// in this case, it should use HEAD^ as baseRef.

	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("stack-%d", i)
		createCommittedStack(name)
	}

	wantRes := RunExpected{
		Stdout: "stack-9\n",
	}

	AssertRunResult(t, ts.ListChangedStacks(), wantRes)
}

func TestFailsOnChangeDetectionIfRepoDoesntHaveOriginMain(t *testing.T) {
	t.Parallel()

	rootdir := test.TempDir(t)
	assertFailsWithChanged := func(stderrRegex string) {
		t.Helper()

		ts := NewCLI(t, rootdir)
		wantRes := RunExpected{
			Status:      1,
			StderrRegex: stderrRegex,
		}

		AssertRunResult(t, ts.ListChangedStacks(), wantRes)

		AssertRunResult(t, ts.Run(
			"run",
			"--changed",
			HelperPath,
			"cat",
			"whatever",
		), wantRes)
	}

	git := sandbox.NewGit(t, rootdir)
	git.InitLocalRepo()

	// list and run works without commit.
	ts := NewCLI(t, rootdir)
	AssertRun(t, ts.ListStacks())
	AssertRun(t, ts.Run("run", "--", HelperPath, "exit", "0"))

	// the main branch only exists after first commit.
	assertFailsWithChanged("flag --changed requires a repository with at least two commits")

	path := test.WriteFile(t, git.BaseDir(), "file1.md", "# generated by terramate")
	git.Add(path)
	git.Commit("first commit")
	// need two commits for changed to make sense.
	assertFailsWithChanged("flag --changed requires a repository with at least two commits")

	path = test.WriteFile(t, git.BaseDir(), "file2.md", "# generated by terramate")
	git.Add(path)
	git.Commit("second commit")

	// have to set default remote explicitly, otherwise fallback to local
	test.WriteFile(t, git.BaseDir(), "terramate.tm.hcl", `
	terramate {
		config {
			git {
				default_remote = "origin"
			}
		}
	}
	`)

	git.SetupRemote("notorigin", "main", "main")
	assertFailsWithChanged("repository must have a configured")

	git.CheckoutNew("not-main")
	git.SetupRemote("origin", "not-main", "main")
	assertFailsWithChanged("has no default branch ")
}

func TestNoArgsProvidesBasicHelp(t *testing.T) {
	t.Parallel()

	cli := NewCLI(t, "")
	help := cli.Run("--help")
	AssertRunResult(t, cli.Run(), RunExpected{Stdout: help.Stdout})
}

func TestFailsIfDefaultRemoteDoesntHaveDefaultBranch(t *testing.T) {
	t.Parallel()

	s := sandbox.NewWithGitConfig(t, sandbox.GitConfig{
		LocalBranchName:         "main",
		DefaultRemoteName:       "origin",
		DefaultRemoteBranchName: "default",
	})

	test.WriteFile(t, s.RootDir(), "terramate.tm.hcl", `
	terramate {
		config {
			git {
				default_remote = "origin"
			}
		}
	}
	`)

	cli := NewCLI(t, s.RootDir())
	AssertRunResult(t,
		cli.ListChangedStacks(),
		RunExpected{
			Status:      1,
			StderrRegex: "has no default branch ",
		},
	)

	test.WriteFile(t, s.RootDir(), "terramate.tm.hcl", `
terramate {
	config {
		git {
			default_remote = "origin"
			default_branch = "default"
		}
	}
}
`)

	AssertRun(t, cli.ListChangedStacks())
}

func TestChangedWorksWithoutRemote(t *testing.T) {
	t.Parallel()

	rootdir := test.TempDir(t)

	git := sandbox.NewGit(t, rootdir)
	git.InitLocalRepo()

	cli := NewCLI(t, git.BaseDir())

	assertSuccess := func(t *testing.T) {
		t.Helper()

		AssertRun(t, cli.ListChangedStacks())

		AssertRun(t, cli.Run(
			"run",
			"--changed",
			HelperPath,
			"cat",
			"whatever",
		))
	}

	AssertRunResult(t,
		cli.ListChangedStacks(),
		RunExpected{
			Status:      1,
			StderrRegex: `requires a repository with at least two commits`,
		},
	)

	path := test.WriteFile(t, git.BaseDir(), "file1", "# generated by terramate")
	git.Add(path)
	git.Commit("first commit")

	AssertRunResult(t,
		cli.ListChangedStacks(),
		RunExpected{
			Status:      1,
			StderrRegex: `requires a repository with at least two commits`,
		},
	)

	path = test.WriteFile(t, git.BaseDir(), "file2", "# generated by terramate")
	git.Add(path)
	git.Commit("second commit")

	assertSuccess(t)

	test.WriteFile(t, git.BaseDir(), "terramate.tm.hcl", `
	terramate {
		config {
			git {
				default_remote = "origin"
			}
		}
	}
	`)

	AssertRunResult(t,
		cli.ListChangedStacks(),
		RunExpected{
			Status:      1,
			StderrRegex: `repository must have a configured "origin" remote`,
		},
	)
}

func TestRemoteSafeguardDefaults(t *testing.T) {
	t.Parallel()

	rootdir := test.TempDir(t)

	git := sandbox.NewGit(t, rootdir)
	git.InitLocalRepo()

	cli := NewCLI(t, git.BaseDir())

	path := test.WriteFile(t, git.BaseDir(), "file1", "# generated by terramate")
	git.Add(path)
	git.Commit("first commit")
	path = test.WriteFile(t, git.BaseDir(), "file2", "# generated by terramate")
	git.Add(path)
	git.Commit("second commit")

	AssertRun(t, cli.ListChangedStacks())

	AssertRun(t, cli.Run(
		"run",
		"--changed",
		HelperPath,
		"cat",
		"whatever",
	))

	git.RemoteAdd("anyremote", "anyurl")

	// List still works
	AssertRun(t, cli.ListChangedStacks())

	// Run fails because of safeguard
	AssertRunResult(t,
		cli.Run(
			"run",
			"--changed",
			HelperPath,
			"cat",
			"whatever",
		),
		RunExpected{
			Status:      1,
			StderrRegex: `'origin' does not appear to be a git repository`,
		},
	)
}

func TestLoadGitRootConfig(t *testing.T) {
	t.Parallel()

	s := sandbox.NewWithGitConfig(t, sandbox.GitConfig{
		DefaultRemoteName:       "mineiros",
		DefaultRemoteBranchName: "default",
		LocalBranchName:         "trunk",
	})

	cli := NewCLI(t, s.RootDir())

	test.WriteFile(t, s.RootDir(), "git.tm.hcl", `
terramate {
	config {
		git {
			default_remote = "mineiros"
			default_branch = "default"
		}
	}
}
`)

	AssertRun(t, cli.ListChangedStacks())
}

func TestE2ETerramateLogsWarningIfRootConfigIsNotAtProjectRoot(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		"s:stacks/stack",
	})

	stacksDir := filepath.Join(s.RootDir(), "stacks")
	test.WriteRootConfig(t, stacksDir)

	tmcli := NewCLI(t, stacksDir)
	tmcli.LogLevel = "warn"
	AssertRunResult(t, tmcli.ListStacks(), RunExpected{
		Stdout:      "stack\n",
		StderrRegex: string(cli.ErrRootCfgInvalidDir),
	})
}

func TestBug515(t *testing.T) {
	t.Parallel()

	// bug: https://github.com/terramate-io/terramate/issues/515

	s := sandbox.NoGit(t, true)
	s.BuildTree([]string{
		"s:stacks/stack",
		"f:common/file.tm",
	})

	stackEntry := s.DirEntry("stacks/stack")
	stackEntry.CreateFile("import.tm", `
		import {
		  source = "/common/file.tm"
		}
	`)

	assertListStacks := func(workdir, want string) {
		t.Helper()

		tmcli := NewCLI(t, workdir)
		AssertRunResult(t, tmcli.ListStacks(), RunExpected{
			Stdout: want,
		})
	}

	assertListStacks(s.RootDir(), "stacks/stack\n")
	assertListStacks(filepath.Join(s.RootDir(), "stacks"), "stack\n")
	assertListStacks(filepath.Join(s.RootDir(), "stacks", "stack"), ".\n")
}

func setupLocalMainBranchBehindOriginMain(git *sandbox.Git, changeFiles func()) {
	// dance below makes local main branch behind origin/main by 1 commit.
	//   - a "temp" branch is created to record current commit.
	//   - go back to main and create 1 additional commit and push to origin/main.
	//   - switch to "temp" and delete "main" reference.
	//   - create "main" branch again based on temp.

	git.CheckoutNew("temp")
	git.Checkout("main")
	changeFiles()
	git.CommitAll("additional commit")
	git.Push("main")
	git.Checkout("temp")
	git.DeleteBranch("main")
	git.CheckoutNew("main")
}
