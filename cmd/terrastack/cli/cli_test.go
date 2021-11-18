package cli_test

import "testing"

func TestBug25(t *testing.T) {
	// bug: https://github.com/mineiros-io/terrastack/issues/25

	te := newTestEnv(t)
	defer te.Cleanup()

	// run will run arbitrary commands ignoring output, checks only success (status=0)
	// it is a little less verbose to just parse a single string, or maybe
	// we go with te.Run("git", "option"), for long list of options that may
	// get annoying and the idea is to approximate a script
	te.Run("git", "init")

	const (
		mod1   = "1"
		mod2   = "2"
		stack1 = "stack-1"
		stack2 = "stack-2"
		stack3 = "stack-3"
	)

	mod1MainTf := te.CreateModule(mod1).CreateFile("main.tf", "# module 1")
	te.CreateModule(mod2).CreateFile("main.tf", "# module 2")

	stack1Handler := te.CreateStack(stack1)
	stack2Handler := te.CreateStack(stack2)
	stack3Handler := te.CreateStack(stack2)

	stack1Handler.CreateFile("main.tf", `
module "mod1" {
    source = "../../modules/1"
}`)
	stack2Handler.CreateFile("main.tf", `
module "mod1" {
    source = "../../modules/2"
}`)
	stack3Handler.CreateFile("main.tf", "# no module")

	// terrastack CLI uses testing.T to fail automatically
	// And also uses the basedir of the test env.
	ts := newTerrastackCLI(t, te.BaseDir())

	// This runs terrastack through the cli.Run main entrypoint
	// no new process is created, automatically validates it succeeded.
	// parameter also parsed so it reads better....but not sure.
	// It should read like in a bash script, just omitting
	// the terrastack command itself since it would be redundant
	for _, s := range []stackHandler{stack1Handler, stack2Handler, stack3Handler} {
		ts.Run("init", s.RelPath())
	}

	te.Run("git", "add", ".")
	te.Run(`git", "commit", "-m", "all"`)

	res := ts.Run("list", "--changed")

	const noChangesOutput = "exact match with expected output"
	if res.Stdout != noChangesOutput {
		t.Errorf("%q got %q, wanted %q", res.Cmd, res.Stdout, noChangesOutput)
		t.Fatalf("%q stderr: %q", res.Cmd, res.Stderr)
	}

	te.Run("git", "checkout", "-b", "change-the-module-1")

	mod1MainTf.Replace("# changed")

	te.Run("git", "add", mod1MainTf.RelPath())
	te.Run("git", "commit", "-m", "module 1 changed")

	res := ts.Run("list", "--changed")

	changedStacks := stack1Handler.RelPath()
	if res.Stdout != changedStacks {
		t.Errorf("%q got %q, wanted %q", res.Cmd, res.Stdout, changedStacks)
		t.Fatalf("%q stderr: %q", res.Cmd, res.Stderr)
	}
}
