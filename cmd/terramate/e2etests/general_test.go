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

package e2etest

import (
	"fmt"
	"testing"

	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestBug25(t *testing.T) {
	// bug: https://github.com/mineiros-io/terramate/issues/25

	const (
		modname1 = "1"
		modname2 = "2"
	)

	t.Parallel()

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
	const (
		modname1 = "1"
		modname2 = "2"
		modname3 = "3"
	)

	t.Parallel()

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
	const (
		mainTfFileName = "main.tf"
		mainTfContents = "# change is the eternal truth of the universe"
	)

	t.Parallel()

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

	cat := test.LookPath(t, "cat")
	wantRun := mainTfContents

	assertRunResult(t, cli.run(
		"run",
		"--changed",
		cat,
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

	cat := test.LookPath(t, "cat")
	wantRun := fmt.Sprintf(
		"Running on changed stacks:\n[%s] running %s %s\n%s\n",
		stack.Path(),
		cat,
		mainTfFileName,
		mainTfContents,
	)

	assertRunResult(t, cli.run(
		"run",
		"--changed",
		cat,
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

func TestFailsOnChangeDetectionIfCurrentBranchIsMainAndItIsOutdated(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)

	stack := s.CreateStack("stack-1")
	mainTfFile := stack.CreateFile("main.tf", "# no code")

	ts := newCLI(t, s.RootDir())

	git := s.Git()
	git.Add(".")
	git.Commit("all")

	// dance below makes makes local main branch behind origin/main by 1 commit.
	//   - a "temp" branch is created to record current commit.
	//   - go back to main and create 1 additional commit and push to origin/main.
	//   - switch to "temp" and delete "main" reference.
	//   - create "main" branch again based on temp.

	git.CheckoutNew("temp")
	git.Checkout("main")
	stack.CreateFile("tempfile", "any content")
	git.CommitAll("additional commit")
	git.Push("main")
	git.Checkout("temp")
	git.DeleteBranch("main")
	git.CheckoutNew("main")

	wantRes := runExpected{
		Status:      1,
		StderrRegex: "main branch is not reachable",
	}

	assertRunResult(t, ts.listChangedStacks(), wantRes)

	cat := test.LookPath(t, "cat")
	assertRunResult(t, ts.run(
		"run",
		"--changed",
		cat,
		mainTfFile.Path(),
	), wantRes)
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

		cat := test.LookPath(t, "cat")
		assertRunResult(t, ts.run(
			"run",
			"--changed",
			cat,
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
