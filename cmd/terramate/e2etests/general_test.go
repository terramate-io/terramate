// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package e2etest

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/terramate-io/terramate/cmd/terramate/cli"
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

	cli := newCLI(t, s.RootDir())
	want := stack1.RelPath() + "\n"
	assertRunResult(t, cli.listChangedStacks(), runExpected{Stdout: want})
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

	cli := newCLI(t, s.RootDir())
	want := stack.RelPath() + "\n"
	assertRunResult(t, cli.listChangedStacks(), runExpected{Stdout: want})
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

	cli := newCLI(t, s.RootDir())

	git := s.Git()
	git.CommitAll("first commit")
	git.Push("main")
	git.CheckoutNew("change-stack")

	stackMainTf.Write(mainTfContents)
	git.CommitAll("stack changed")

	wantList := stack.RelPath() + "\n"
	assertRunResult(t, cli.listChangedStacks(), runExpected{Stdout: wantList})

	wantRun := mainTfContents

	assertRunResult(t, cli.run(
		"run",
		"--changed",
		testHelperBin,
		"cat",
		mainTfFileName,
	), runExpected{
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

	cli := newCLI(t, t.TempDir())

	git := s.Git()
	git.CommitAll("first commit")
	git.Push("main")
	git.CheckoutNew("change-stack")

	stackMainTf.Write(mainTfContents)
	git.CommitAll("stack changed")

	wantList := stack.Path() + "\n"
	assertRunResult(t, cli.listChangedStacks(), runExpected{Stdout: wantList})

	wantRun := fmt.Sprintf(
		"Running on changed stacks:\n[%s] running %s %s %s\n%s\n",
		stack.Path(),
		testHelperBin,
		"cat",
		mainTfFileName,
		mainTfContents,
	)

	assertRunResult(t, cli.run(
		"run",
		"--changed",
		testHelperBin,
		"cat",
		mainTfFileName,
	), runExpected{Stdout: wantRun})
}

func TestDefaultBaseRefInOtherThanMain(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)

	stack := s.CreateStack("stack-1")
	stackFile := stack.CreateFile("main.tf", "# no code")

	cli := newCLI(t, s.RootDir())

	git := s.Git()
	git.Add(".")
	git.Commit("all")
	git.Push("main")
	git.CheckoutNew("change-the-stack")

	stackFile.Write("# changed")
	git.Add(stack.Path())
	git.Commit("stack changed")

	want := runExpected{
		Stdout: stack.RelPath() + "\n",
	}
	assertRunResult(t, cli.listChangedStacks(), want)
}

func TestDefaultBaseRefInMain(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)

	stack := s.CreateStack("stack-1")
	stack.CreateFile("main.tf", "# no code")

	cli := newCLI(t, s.RootDir())

	git := s.Git()
	git.Add(".")
	git.Commit("all")
	git.Push("main")

	// main uses HEAD^ as default baseRef.
	want := runExpected{
		Stdout: stack.RelPath() + "\n",
	}
	assertRunResult(t, cli.listChangedStacks(), want)
}

func TestBaseRefFlagPrecedenceOverDefault(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)

	stack := s.CreateStack("stack-1")
	stack.CreateFile("main.tf", "# no code")

	cli := newCLI(t, s.RootDir())

	git := s.Git()
	git.Add(".")
	git.Commit("all")
	git.Push("main")

	assertRunResult(t, cli.listChangedStacks("--git-change-base", "origin/main"),
		runExpected{
			IgnoreStderr: true,
		},
	)
}

func TestMainAfterOriginMainMustUseDefaultBaseRef(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	ts := newCLI(t, s.RootDir())

	createCommittedStack := func(name string) {
		stack := s.CreateStack(name)
		stack.CreateFile("main.tf", "# no code")

		git := s.Git()
		git.Add(".")
		git.Commit(name)
	}

	wantStdout := ""

	// creates N commits in main.
	// in this case, it should use origin/main as baseRef even if in main.

	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("stack-%d", i)
		createCommittedStack(name)
		wantStdout += name + "\n"
	}

	wantRes := runExpected{
		Stdout: wantStdout,
	}

	assertRunResult(t, ts.listChangedStacks(), wantRes)
}

func TestFailsOnChangeDetectionIfRepoDoesntHaveOriginMain(t *testing.T) {
	t.Parallel()

	rootdir := t.TempDir()
	assertFails := func(stderrRegex string) {
		t.Helper()

		ts := newCLI(t, rootdir)
		wantRes := runExpected{
			Status:      1,
			StderrRegex: stderrRegex,
		}

		assertRunResult(t, ts.listChangedStacks(), wantRes)

		assertRunResult(t, ts.run(
			"run",
			"--changed",
			testHelperBin,
			"cat",
			"whatever",
		), wantRes)
	}

	git := sandbox.NewGit(t, rootdir)
	git.InitLocalRepo()

	assertFails("repository must have a configured")

	// the main branch only exists after first commit.
	path := test.WriteFile(t, git.BaseDir(), "README.md", "# generated by terramate")
	git.Add(path)
	git.Commit("first commit")

	git.SetupRemote("notorigin", "main", "main")
	assertFails("repository must have a configured")

	git.CheckoutNew("not-main")
	git.SetupRemote("origin", "not-main", "main")
	assertFails("has no default branch ")
}

func TestNoArgsProvidesBasicHelp(t *testing.T) {
	t.Parallel()

	cli := newCLI(t, "")
	help := cli.run("--help")
	assertRunResult(t, cli.run(), runExpected{Stdout: help.Stdout})
}

func TestFailsIfDefaultRemoteDoesntHaveDefaultBranch(t *testing.T) {
	t.Parallel()

	s := sandbox.NewWithGitConfig(t, sandbox.GitConfig{
		LocalBranchName:         "main",
		DefaultRemoteName:       "origin",
		DefaultRemoteBranchName: "default",
	})

	cli := newCLI(t, s.RootDir())
	assertRunResult(t,
		cli.listChangedStacks(),
		runExpected{
			Status:      1,
			StderrRegex: "has no default branch ",
		},
	)

	test.WriteFile(t, s.RootDir(), "terramate.tm.hcl", `
terramate {
	config {
		git {
			default_branch = "default"
		}
	}
}
`)

	assertRun(t, cli.listChangedStacks())
}

func TestLoadGitRootConfig(t *testing.T) {
	t.Parallel()

	s := sandbox.NewWithGitConfig(t, sandbox.GitConfig{
		DefaultRemoteName:       "mineiros",
		DefaultRemoteBranchName: "default",
		LocalBranchName:         "trunk",
	})

	cli := newCLI(t, s.RootDir())

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

	assertRun(t, cli.listChangedStacks())
}

func TestE2ETerramateLogsWarningIfRootConfigIsNotAtProjectRoot(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		"s:stacks/stack",
	})

	stacksDir := filepath.Join(s.RootDir(), "stacks")
	test.WriteRootConfig(t, stacksDir)

	tmcli := newCLI(t, stacksDir)
	tmcli.loglevel = "warn"
	assertRunResult(t, tmcli.listStacks(), runExpected{
		Stdout:      "stack\n",
		StderrRegex: string(cli.ErrRootCfgInvalidDir),
	})
}

func TestBug515(t *testing.T) {
	t.Parallel()

	// bug: https://github.com/terramate-io/terramate/issues/515

	s := sandbox.New(t)
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

		tmcli := newCLI(t, workdir)
		assertRunResult(t, tmcli.listStacks(), runExpected{
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
