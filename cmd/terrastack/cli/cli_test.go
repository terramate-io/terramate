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

	te := sandbox.New(t)

	mod1MainTf := te.CreateModule(mod1).CreateFile("main.tf", "# module 1")
	te.CreateModule(mod2).CreateFile("main.tf", "# module 2")

	stack1 := te.CreateStack("stack-1")
	stack2 := te.CreateStack("stack-2")
	stack3 := te.CreateStack("stack-3")

	stack1.CreateFile("main.tf", `
module "mod1" {
source = "%s"
}`, stack1.ModSource(mod1))

	stack2.CreateFile("main.tf", `
module "mod2" {
source = "%s"
}`, stack2.ModSource(mod2))

	stack3.CreateFile("main.tf", "# no module")

	cli := newCLI(t)
	cli.run("init", stack1.Path(), stack2.Path(), stack3.Path())

	git := te.Git()
	git.CommitAll("first commit")

	cli.run("list", te.BaseDir(), "--changed").HasStdout("")

	git.CheckoutNew("change-the-module-1")

	mod1MainTf.Write("# changed")

	git.CommitAll("module 1 changed")

	want := stack1.Path() + "\n"
	cli.run("list", te.BaseDir(), "--changed").HasStdout(want)
}

func TestListAndRunChangedStack(t *testing.T) {
	const (
		mainTfFileName = "main.tf"
		mainTfContents = "# change is the eternal truth of the universe"
	)

	te := sandbox.New(t)

	stack := te.CreateStack("stack")
	stackMainTf := stack.CreateFile(mainTfFileName, "# some code")

	cli := newCLI(t)
	cli.run("init", stack.Path())

	git := te.Git()
	git.CommitAll("first commit")

	cli.run("list", te.BaseDir(), "--changed").HasStdout("")

	git.CheckoutNew("change-stack")

	stackMainTf.Write(mainTfContents)
	git.CommitAll("stack changed")

	wantList := stack.Path() + "\n"
	cli.run("list", te.BaseDir(), "--changed").HasStdout(wantList)

	cat := test.LookPath(t, "cat")
	wantRun := fmt.Sprintf(`Running on changed stacks:
[%s] running %s %s
# change is the eternal truth of the universe`, stack.Path(), cat, mainTfFileName)
	cli.run(
		"run",
		"--basedir",
		te.BaseDir(),
		"--changed",
		cat,
		mainTfFileName,
	).HasStdout(wantRun)
}

func TestNoArgsProvidesBasicHelp(t *testing.T) {
	cli := newCLI(t)
	help := cli.run("--help")
	cli.run().HasStdout(help.Stdout)
}

type runResult struct {
	t      *testing.T
	Cmd    string
	Stdout string
	Stderr string
}

type tscli struct {
	t *testing.T
}

func newCLI(t *testing.T) tscli {
	return tscli{t: t}
}

func (ts tscli) run(args ...string) runResult {
	ts.t.Helper()

	stdin := &bytes.Buffer{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	if err := cli.Run(args, stdin, stdout, stderr); err != nil {
		ts.t.Fatalf(
			"cli.Run(args=%v) error=%q stdout=%q stderr=%q",
			args,
			err,
			stdout.String(),
			stderr.String(),
		)
	}

	return runResult{
		t:      ts.t,
		Cmd:    strings.Join(args, " "),
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
}

func (res runResult) HasStdout(want string) {
	res.t.Helper()

	if res.Stdout != want {
		res.t.Errorf("%q stdout=%q, wanted=%q", res.Cmd, res.Stdout, want)
		res.t.Fatalf("%q stderr=%q", res.Cmd, res.Stderr)
	}
}
