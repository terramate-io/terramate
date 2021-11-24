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
	git.Push("main")

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

func TestDefaultBaseRef(t *testing.T) {
	s := sandbox.New(t)

	stack := s.CreateStack("stack-1")
	stackFile := stack.CreateFile("main.tf", "# no code")

	tsrun(t, "init", stack.Path())

	git := s.Git()
	git.Add(".")
	git.Commit("all")
	git.Push("main")

	res := tsrun(t, "list", s.BaseDir(), "--changed")

	const noChangesOutput = ""
	if res.Stdout != noChangesOutput {
		t.Errorf("%q stdout=%q, wanted=%q", res.Cmd, res.Stdout, noChangesOutput)
		t.Fatalf("%q stderr=%q", res.Cmd, res.Stderr)
	}

	git.Checkout("change-the-stack", true)

	stackFile.Write("# changed")
	git.Add(stack.Path())
	git.Commit("stack changed")

	res = tsrun(t, "list", s.BaseDir(), "--changed")

	changedStacks := stack.Path() + "\n"
	if res.Stdout != changedStacks {
		t.Errorf("%q stdout=%q, wanted=%q", res.Cmd, res.Stdout, changedStacks)
		t.Fatalf("%q stderr=%q", res.Cmd, res.Stderr)
	}

	git.Checkout("main", false)
	git.Merge("change-the-stack")
	git.Push("main")

	res = tsrun(t, "list", s.BaseDir(), "--changed")
	if res.Stdout != noChangesOutput {
		t.Errorf("%q stdout=%q, wanted=%q", res.Cmd, res.Stdout, noChangesOutput)
		t.Fatalf("%q stderr=%q", res.Cmd, res.Stderr)
	}
}

type runResult struct {
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
		Cmd:    strings.Join(args, " "),
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
}
