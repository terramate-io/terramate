package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mineiros-io/terrastack/cmd/terrastack/cli"
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

	tsrun(t, "init", stack1.Path(), stack2.Path(), stack3.Path())

	git := te.Git()
	git.Add(".")
	git.Commit("all")

	res := tsrun(t, "list", te.BaseDir(), "--changed")

	const noChangesOutput = ""
	if res.Stdout != noChangesOutput {
		t.Errorf("%q stdout=%q, wanted=%q", res.Cmd, res.Stdout, noChangesOutput)
		t.Fatalf("%q stderr=%q", res.Cmd, res.Stderr)
	}

	git.Checkout("change-the-module-1", true)

	mod1MainTf.Write("# changed")

	git.Add(mod1MainTf.Path())
	git.Commit("module 1 changed")

	res = tsrun(t, "list", te.BaseDir(), "--changed")

	changedStacks := stack1.Path() + "\n"

	if res.Stdout != changedStacks {
		t.Errorf("%q stdout=%q, wanted=%q", res.Cmd, res.Stdout, changedStacks)
		t.Fatalf("%q stderr=%q", res.Cmd, res.Stderr)
	}
}

func TestListChangedStack(t *testing.T) {
	te := sandbox.New(t)

	stack := te.CreateStack("stack")
	stackMainTf := stack.CreateFile("main.tf", "# some code")

	tsrun(t, "init", stack.Path())

	git := te.Git()
	git.Add(".")
	git.Commit("all")

	tsrun(t, "list", te.BaseDir(), "--changed").HasStdout("")

	git.Checkout("change-stack", true)

	stackMainTf.Write("# change is the eternal truth of the universe")

	git.Add(stackMainTf.Path())
	git.Commit("stack changed")

	want := stack.Path() + "\n"

	tsrun(t, "list", te.BaseDir(), "--changed").HasStdout(want)
}

type runResult struct {
	t      *testing.T
	Cmd    string
	Stdout string
	Stderr string
}

func tsrun(t *testing.T, args ...string) runResult {
	t.Helper()

	stdin := &bytes.Buffer{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	if err := cli.Run(args, stdin, stdout, stderr); err != nil {
		t.Fatalf(
			"cli.Run(args=%v) error=%q stdout=%q stderr=%q",
			args,
			err,
			stdout.String(),
			stderr.String(),
		)
	}

	return runResult{
		t:      t,
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
