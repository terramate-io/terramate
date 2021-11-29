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
	assertRun(t, cli.run("init", basedir))

	data := test.ReadFile(t, basedir, terrastack.ConfigFilename)
	p := hhcl.NewParser()
	got, err := p.Parse("TestInitHCL", data)
	assert.NoError(t, err, "parsing terrastack file")

	want := hcl.Terrastack{
		RequiredVersion: terrastack.Version(),
	}
	if *got != want {
		t.Fatalf("terrastack file differs: %+v != %+v", want, *got)
	}
}

func TestInitHCLFileAlreadyExists(t *testing.T) {
	basedir := t.TempDir()

	c := newCLI(t, basedir)
	assertRun(t, c.run("init", basedir))

	data := test.ReadFile(t, basedir, terrastack.ConfigFilename)
	p := hhcl.NewParser()
	got, err := p.Parse("TestInitHCL", data)
	assert.NoError(t, err, "parsing terrastack file")

	want := hcl.Terrastack{
		RequiredVersion: terrastack.Version(),
	}
	if *got != want {
		t.Fatalf("terrastack file differs: %+v != %+v", want, *got)
	}

	// same version, must work
	assertRun(t, c.run("init", basedir))

	// different version must fail and give a warning.

	stackVersion := "99999.99999.99999"
	test.WriteFile(t, basedir, terrastack.ConfigFilename, fmt.Sprintf(`
terrastack {
	required_version = %q
}`, stackVersion))

	wantResult := runResult{
		Error:        cli.ErrInit,
		IgnoreStderr: true,
	}
	assertRunResult(t, c.run("init", basedir), wantResult)
}
