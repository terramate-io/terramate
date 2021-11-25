package cli_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/mineiros-io/terrastack/cmd/terrastack/cli"
	"github.com/mineiros-io/terrastack/test"
	"github.com/mineiros-io/terrastack/test/sandbox"
)

func TestBug25(t *testing.T) {
	// bug: https://github.com/mineiros-io/terrastack/issues/25

	const (
		mod1 = "1"
		mod2 = "2"
	)

	s := sandbox.New(t)

	mod1MainTf := s.CreateModule(mod1).CreateFile("main.tf", "# module 1")
	s.CreateModule(mod2).CreateFile("main.tf", "# module 2")

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

	cli := newCLI(t, s.BaseDir())
	cli.run("init", stack1.Path(), stack2.Path(), stack3.Path())

	git := s.Git()
	git.CommitAll("first commit")
	git.Push("main")
	git.CheckoutNew("change-the-module-1")

	mod1MainTf.Write("# changed")

	git.CommitAll("module 1 changed")

	want := stack1.RelPath() + "\n"
	assertRun(t, cli.run(
		"list", s.BaseDir(), "--changed"),
		runResult{Stdout: want},
	)
}

func TestListAndRunChangedStack(t *testing.T) {
	const (
		mainTfFileName = "main.tf"
		mainTfContents = "# change is the eternal truth of the universe"
	)

	s := sandbox.New(t)

	stack := s.CreateStack("stack")
	stackMainTf := stack.CreateFile(mainTfFileName, "# some code")

	cli := newCLI(t, s.BaseDir())
	cli.run("init", stack.Path())

	git := s.Git()
	git.CommitAll("first commit")
	git.Push("main")
	git.CheckoutNew("change-stack")

	stackMainTf.Write(mainTfContents)
	git.CommitAll("stack changed")

	wantList := stack.RelPath() + "\n"
	assertRun(t, cli.run("list", s.BaseDir(), "--changed"), runResult{Stdout: wantList})

	cat := test.LookPath(t, "cat")
	wantRun := fmt.Sprintf(
		"Running on changed stacks:\n[%s] running %s %s\n%s",
		stack.Path(),
		cat,
		mainTfFileName,
		mainTfContents,
	)

	assertRun(t, cli.run(
		"run",
		"--basedir",
		s.BaseDir(),
		"--changed",
		cat,
		mainTfFileName,
	), runResult{Stdout: wantRun})
}

func TestDefaultBaseRefInOtherThanMain(t *testing.T) {
	s := sandbox.New(t)

	stack := s.CreateStack("stack-1")
	stackFile := stack.CreateFile("main.tf", "# no code")

	cli := newCLI(t, s.BaseDir())
	assertRun(t, cli.run("init", stack.Path()), runResult{})

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
	assertRun(t, cli.run("list", s.BaseDir(), "--changed"), want)
}

func TestDefaultBaseRefInMain(t *testing.T) {
	s := sandbox.New(t)

	stack := s.CreateStack("stack-1")
	stack.CreateFile("main.tf", "# no code")

	cli := newCLI(t, s.BaseDir())
	assertRun(t, cli.run("init", stack.Path()), runResult{})

	git := s.Git()
	git.Add(".")
	git.Commit("all")
	git.Push("main")

	// main uses HEAD^1 as default baseRef.

	want := runResult{
		Stdout: stack.RelPath() + "\n",
	}
	assertRun(t, cli.run("list", s.BaseDir(), "--changed"), want)
}

func TestBaseRefFlagPrecedenceOverDefault(t *testing.T) {
	s := sandbox.New(t)

	stack := s.CreateStack("stack-1")
	stack.CreateFile("main.tf", "# no code")

	cli := newCLI(t, s.BaseDir())
	assertRun(t, cli.run("init", stack.Path()), runResult{})

	git := s.Git()
	git.Add(".")
	git.Commit("all")
	git.Push("main")

	want := runResult{
		Stdout: stack.RelPath() + "\n",
	}
	assertRun(t, cli.run("list", s.BaseDir(), "--changed"), want)
	assertRun(t, cli.run(
		"--git-change-base", "origin/main", "list", s.BaseDir(), "--changed",
	), runResult{})
}

func TestNoArgsProvidesBasicHelp(t *testing.T) {
	cli := newCLI(t, t.TempDir())
	cli.run("--help")
	help := cli.run("--help")
	assertRun(t, cli.run(), runResult{Stdout: help.Stdout})
}

type runResult struct {
	Cmd    string
	Stdout string
	Stderr string
}

type tscli struct {
	t  *testing.T
	wd string
}

func newCLI(t *testing.T, wd string) tscli {
	return tscli{
		t:  t,
		wd: wd,
	}
}

func (ts tscli) run(args ...string) runResult {
	ts.t.Helper()

	stdin := &bytes.Buffer{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	if err := cli.Run(ts.wd, args, stdin, stdout, stderr); err != nil {
		ts.t.Fatalf(
			"cli.Run(args=%v) error=%q stdout=%q stderr=%q",
			args,
			err,
			stdout.String(),
			stderr.String(),
		)
	}

	return runResult{
		Cmd:    strings.Join(args, " "),
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
}

func assertRun(t *testing.T, got runResult, want runResult) {
	t.Helper()

	if got.Stdout != want.Stdout {
		t.Errorf("%q stdout=%q, wanted=%q", got.Cmd, got.Stdout, want.Stdout)
	}

	if got.Stderr != want.Stderr {
		t.Errorf("%q stderr=%q, wanted=%q", got.Cmd, got.Stderr, want.Stderr)
	}
}
