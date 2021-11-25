package cli_test

import (
	"fmt"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terrastack"
	"github.com/mineiros-io/terrastack/cmd/terrastack/cli"
	"github.com/mineiros-io/terrastack/hcl"
	"github.com/mineiros-io/terrastack/hcl/hhcl"
	"github.com/mineiros-io/terrastack/test"
)

func TestInitHCLFile(t *testing.T) {
	basedir := t.TempDir()

	cli := newCLI(t, basedir)
	assertRun(t, cli.run("init", basedir), runResult{})

	data := test.ReadFile(t, basedir, terrastack.ConfigFilename)
	p := hhcl.NewTSParser()
	got, err := p.Parse("TestInitHCL", data)
	assert.NoError(t, err, "parsing terrastack file")

	want := hcl.Terrastack{
		RequiredVersion: terrastack.Version(),
	}
	if *got != want {
		t.Fatalf("terrastack file differs: %+v != %+v", want, got)
	}
}

func TestInitHCLFileAlreadyExists(t *testing.T) {
	basedir := t.TempDir()

	c := newCLI(t, basedir)
	assertRun(t, c.run("init", basedir), runResult{})

	data := test.ReadFile(t, basedir, terrastack.ConfigFilename)
	p := hhcl.NewTSParser()
	got, err := p.Parse("TestInitHCL", data)
	assert.NoError(t, err, "parsing terrastack file")

	want := hcl.Terrastack{
		RequiredVersion: terrastack.Version(),
	}
	if *got != want {
		t.Fatalf("terrastack file differs: %+v != %+v", want, got)
	}

	// same version, must work
	assertRun(t, c.run("init", basedir), runResult{})

	// different version must fail and give a warning.
	test.WriteFile(t, basedir, terrastack.ConfigFilename, fmt.Sprintf(`
terrastack {
	required_version = %q
}`, "99999.99999.99999"))

	wantResult := runResult{
		Err: cli.ErrInit,
		Stderr: "warn: failed to initialize stack: stack already initialized with " +
			"version \"99999.99999.99999\" but terrastack version is \"0.0.1\"\n",
	}
	assertRun(t, c.runFail("init", basedir), wantResult)
}
