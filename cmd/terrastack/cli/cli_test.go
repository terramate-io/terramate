package cli_test

import "testing"

func TestBug25(t *testing.T) {
	// bug: https://github.com/mineiros-io/terrastack/issues/25

	te := newTestEnv(t)
	// run will run arbitrary commands ignoring output, checks only success (status=0)
	// it is a little less verbose to just parse a single string, or maybe
	// we go with te.Run("git", "option"), for long list of options that may
	// get annoying and the idea is to approximate a script
	te.Run("git init")

	// Multiline handling is always messy, still prefer this than
	// bash, but maybe it is just me =P.
	te.CreateFiles([]File{
		{Path: "modules/1/main.tf", Body: "# module 1"},
		{Path: "modules/2/main.tf", Body: "# module 2"},
		{Path: "stacks/stack-1/main.tf", Body: `
module "mod1" {
    source = "../../modules/1"
}`},
		{Path: "stacks/stack-2/main.tf", Body: `
module "mod1" {
    source = "../../modules/2"
}`},
		{Path: "stacks/stack-3/main.tf", Body: "# no module"},
	})

	// terrastack CLI uses testing.T to fail automatically
	// And also uses the basedir of the test env.
	ts := newTerrastackCLI(t, te.BaseDir)

	// This runs terrastack through the cli.Run main entrypoint
	// no new process is created, automatically validates it succeeded.
	// parameter also parsed so it reads better....but not sure.
	// It should read like in a bash script, just omitting
	// the terrastack command itself since it would be redundant
	ts.Run("init stacks/stack-1")
	ts.Run("init stacks/stack-2")
	ts.Run("init stacks/stack-3")

	te.Run("git add .")
	te.Run(`git commit -m "all"`)

	res := ts.Run("list --changed")

	const noChangesOutput = "exact match with expected output"
	if res.Stdout != noChangesOutput {
		t.Errorf("%q got %q, wanted %q", res.Cmd, res.Stdout, noChangesOutput)
		t.Fatalf("%q stderr: %q", res.Cmd, res.Stderr)
	}

	te.Run("git checkout -b change-the-module-1")
	te.CreateFile("modules/1/main.tf", "# changed")
	te.Run("git add modules/1/main.tf")
	te.Run(`git commit -m "module 1 changed"`)

	res := ts.Run("list --changed")

	const outputWithChanges = "exact match with expected output"
	if res.Stdout != outputWithChanges {
		t.Errorf("%q got %q, wanted %q", res.Cmd, res.Stdout, outputWithChanges)
		t.Fatalf("%q stderr: %q", res.Cmd, res.Stderr)
	}
}
