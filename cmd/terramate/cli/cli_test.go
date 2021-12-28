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

package cli_test

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mineiros-io/terramate/cmd/terramate/cli"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestBug25(t *testing.T) {
	// bug: https://github.com/mineiros-io/terramate/issues/25

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
	assertRunResult(t, cli.run("list", "--changed"), runResult{Stdout: want})
}

func TestBugModuleMultipleFilesSameDir(t *testing.T) {
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
	assertRunResult(t, cli.run("list", "--changed"), runResult{Stdout: want})
}

func TestListAndRunChangedStack(t *testing.T) {
	const (
		mainTfFileName = "main.tf"
		mainTfContents = "# change is the eternal truth of the universe"
	)

	s := sandbox.New(t)

	stack := s.CreateStack("stack")
	stackMainTf := stack.CreateFile(mainTfFileName, "# some code")

	cli := newCLI(t, s.RootDir())
	cli.run("init", stack.Path())

	git := s.Git()
	git.CommitAll("first commit")
	git.Push("main")
	git.CheckoutNew("change-stack")

	stackMainTf.Write(mainTfContents)
	git.CommitAll("stack changed")

	wantList := stack.RelPath() + "\n"
	assertRunResult(t, cli.run("list", "--changed"), runResult{Stdout: wantList})

	cat := test.LookPath(t, "cat")
	wantRun := fmt.Sprintf(
		"Running on changed stacks:\n[%s] running %s %s\n%s\n",
		stack.RelPath(),
		cat,
		mainTfFileName,
		mainTfContents,
	)

	assertRunResult(t, cli.run(
		"run",
		"--changed",
		cat,
		mainTfFileName,
	), runResult{Stdout: wantRun})
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
	cli.run("init", stack.Path())

	git := s.Git()
	git.CommitAll("first commit")
	git.Push("main")
	git.CheckoutNew("change-stack")

	stackMainTf.Write(mainTfContents)
	git.CommitAll("stack changed")

	wantList := stack.Path() + "\n"
	assertRunResult(t, cli.run("list", "--changed"), runResult{Stdout: wantList})

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
	), runResult{Stdout: wantRun})
}

func TestDefaultBaseRefInOtherThanMain(t *testing.T) {
	s := sandbox.New(t)

	stack := s.CreateStack("stack-1")
	stackFile := stack.CreateFile("main.tf", "# no code")

	cli := newCLI(t, s.RootDir())
	assertRun(t, cli.run("init", stack.Path()))

	git := s.Git()
	git.Add(".")
	git.Commit("all")
	git.Push("main")
	git.CheckoutNew("change-the-stack")

	stackFile.Write("# changed")
	git.Add(stack.Path())
	git.Commit("stack changed")

	want := runResult{
		Stdout: stack.RelPath() + "\n",
	}
	assertRunResult(t, cli.run("list", "--changed"), want)
}

func TestDefaultBaseRefInMain(t *testing.T) {
	s := sandbox.New(t)

	stack := s.CreateStack("stack-1")
	stack.CreateFile("main.tf", "# no code")

	cli := newCLI(t, s.RootDir())
	assertRun(t, cli.run("init", stack.Path()))

	git := s.Git()
	git.Add(".")
	git.Commit("all")
	git.Push("main")

	// main uses HEAD^1 as default baseRef.
	want := runResult{
		Stdout:       stack.RelPath() + "\n",
		IgnoreStderr: true,
	}
	assertRunResult(t, cli.run("list", "--changed"), want)
}

func TestBaseRefFlagPrecedenceOverDefault(t *testing.T) {
	s := sandbox.New(t)

	stack := s.CreateStack("stack-1")
	stack.CreateFile("main.tf", "# no code")

	cli := newCLI(t, s.RootDir())
	assertRun(t, cli.run("init", stack.Path()))

	git := s.Git()
	git.Add(".")
	git.Commit("all")
	git.Push("main")

	assertRunResult(t, cli.run("list", "--changed",
		"--git-change-base", "origin/main"),
		runResult{
			IgnoreStderr: true,
		},
	)
}

func TestFailsOnChangeDetectionIfCurrentBranchIsMainAndItIsOutdated(t *testing.T) {
	s := sandbox.New(t)

	stack := s.CreateStack("stack-1")
	mainTfFile := stack.CreateFile("main.tf", "# no code")

	ts := newCLI(t, s.RootDir())
	assertRun(t, ts.run("init", stack.Path()))

	git := s.Git()
	git.Add(".")
	git.Commit("all")

	wantRes := runResult{
		Error:        cli.ErrOutdatedLocalRev,
		IgnoreStderr: true,
	}

	assertRunResult(t, ts.run("list", "--changed"), wantRes)

	cat := test.LookPath(t, "cat")
	assertRunResult(t, ts.run(
		"run",
		"--changed",
		cat,
		mainTfFile.Path(),
	), wantRes)
}

func TestFailsOnChangeDetectionIfRepoDoesntHaveOriginMain(t *testing.T) {
	rootdir := t.TempDir()
	assertFails := func() {
		t.Helper()

		ts := newCLI(t, rootdir)
		wantRes := runResult{
			Error:        cli.ErrNoDefaultRemoteConfig,
			IgnoreStderr: true,
		}

		assertRunResult(t, ts.run("list", "--changed"), wantRes)

		cat := test.LookPath(t, "cat")
		assertRunResult(t, ts.run(
			"run",
			"--changed",
			cat,
			"whatever",
		), wantRes)
	}

	git := sandbox.NewGit(t, rootdir)
	git.InitBasic()

	assertFails()

	// the main branch only exists after first commit.
	path := test.WriteFile(t, git.BaseDir(), "README.md", "# generated by terramate")
	git.Add(path)
	git.Commit("first commit")

	git.SetupRemote("notorigin", "main")
	assertFails()

	git.CheckoutNew("not-main")
	git.SetupRemote("origin", "not-main")
	assertFails()
}

func TestNoArgsProvidesBasicHelp(t *testing.T) {
	cli := newCLI(t, "")
	cli.run("--help")
	help := cli.run("--help")
	assertRunResult(t, cli.run(), runResult{Stdout: help.Stdout})
}

type runResult struct {
	Cmd           string
	Stdout        string
	FlattenStdout bool
	IgnoreStdout  bool
	Stderr        string
	IgnoreStderr  bool
	Error         error
}

type tscli struct {
	t     *testing.T
	chdir string
}

func newCLI(t *testing.T, chdir string) tscli {
	return tscli{
		t:     t,
		chdir: chdir,
	}
}

func (ts tscli) run(args ...string) runResult {
	ts.t.Helper()

	stdin := &bytes.Buffer{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	allargs := []string{}
	if ts.chdir != "" {
		allargs = append(allargs, "--chdir", ts.chdir)
	}

	allargs = append(allargs, args...)
	err := cli.Run(allargs, false, stdin, stdout, stderr)

	return runResult{
		Cmd:    strings.Join(args, " "),
		Stdout: stdout.String(),
		Stderr: stderr.String(),
		Error:  err,
	}
}

func assertRun(t *testing.T, got runResult) {
	t.Helper()

	assertRunResult(t, got, runResult{IgnoreStdout: true, IgnoreStderr: true})
}

func assertRunResult(t *testing.T, got runResult, want runResult) {
	t.Helper()

	stdout := got.Stdout
	wantStdout := want.Stdout
	if want.FlattenStdout {
		stdout = flatten(stdout)
		wantStdout = flatten(wantStdout)
	}

	if !want.IgnoreStdout && stdout != wantStdout {
		t.Errorf("%q stdout=\"%s\" != wanted=\"%s\"", got.Cmd, stdout, wantStdout)
	}

	if !want.IgnoreStderr && got.Stderr != want.Stderr {
		t.Errorf("%q stderr=\"%s\" != wanted=\"%s\"", got.Cmd, got.Stderr, want.Stderr)
	}

	if !errors.Is(got.Error, want.Error) {
		t.Errorf("%q got.Error=[%v] != want.Error=[%v]", got.Cmd, got.Error, want.Error)
	}
}
