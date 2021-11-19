package cli_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/mineiros-io/terrastack/cmd/terrastack/cli"
)

func TestBug25(t *testing.T) {
	// bug: https://github.com/mineiros-io/terrastack/issues/25

	const (
		mod1   = "1"
		mod2   = "2"
		stack1 = "stack-1"
		stack2 = "stack-2"
		stack3 = "stack-3"
	)

	te := NewTestEnv(t)
	defer te.Cleanup()

	mod1MainTf := te.CreateModule(mod1).CreateFile("main.tf", "# module 1")
	te.CreateModule(mod2).CreateFile("main.tf", "# module 2")

	stack1Entry := te.CreateStack(stack1)
	stack2Entry := te.CreateStack(stack2)
	stack3Entry := te.CreateStack(stack3)

	stack1Entry.CreateFile("main.tf", `
module "mod1" {
source = "%s"
}`, stack1Entry.ModImportPath(mod1))

	stack2Entry.CreateFile("main.tf", `
module "mod2" {
source = "%s"
}`, stack2Entry.ModImportPath(mod2))

	stack3Entry.CreateFile("main.tf", "# no module")

	tscli := NewCLI(t, te.BaseDir())

	tscli.Run("init", stack1Entry.Path(), stack2Entry.Path(), stack3Entry.Path())

	git := te.Git()
	git.Add(".")
	git.Commit("all")

	res := tscli.Run("list", "--changed")

	const noChangesOutput = ""
	if res.Stdout != noChangesOutput {
		t.Errorf("%q stdout=%q, wanted=%q", res.Cmd, res.Stdout, noChangesOutput)
		t.Fatalf("%q stderr=%q", res.Cmd, res.Stderr)
	}

	git.Checkout("change-the-module-1", true)

	mod1MainTf.Write("# changed")

	git.Add(mod1MainTf.Path())
	git.Commit("module 1 changed")

	t.Log(os.Getwd())
	res = tscli.Run("list", "--changed")

	changedStacks := stack1Entry.Path() + "\n"

	if res.Stdout != changedStacks {
		t.Errorf("%q stdout=%q, wanted=%q", res.Cmd, res.Stdout, changedStacks)
		t.Fatalf("%q stderr=%q", res.Cmd, res.Stderr)
	}
}

type TestCLI struct {
	t       *testing.T
	basedir string
}

type CLIRunResult struct {
	Cmd    string
	Stdout string
	Stderr string
}

func NewCLI(t *testing.T, basedir string) *TestCLI {
	return &TestCLI{t: t, basedir: basedir}
}

func (tc *TestCLI) Run(args ...string) CLIRunResult {
	tc.t.Helper()

	stdin := &bytes.Buffer{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	if err := cli.Run(args, tc.basedir, stdin, stdout, stderr); err != nil {
		tc.t.Errorf(
			"cli.Run(args=%v, basedir=%s) error=%q stdout=%q stderr=%q",
			args,
			tc.basedir,
			err,
			stdout.String(),
			stderr.String(),
		)
	}

	return CLIRunResult{
		Cmd:    strings.Join(args, " "),
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
}
