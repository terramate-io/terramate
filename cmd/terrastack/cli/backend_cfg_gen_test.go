package cli_test

import (
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terrastack"
	"github.com/mineiros-io/terrastack/hcl"
	"github.com/mineiros-io/terrastack/test/sandbox"
)

func TestBackendConfigOnLeafSingleStack(t *testing.T) {
	s := sandbox.New(t)
	stack := s.CreateStack("stack")

	backendBlock := `backend "type" {
		param = "value"
	}`

	stack.CreateConfig(`terrastack {
  %s
  %s
}`, versionAttribute(), backendBlock)

	ts := newCLI(t, s.BaseDir())
	assertRunResult(t, ts.run("generate"), runResult{IgnoreStdout: true})

	got := stack.ReadGeneratedTf()

	if !strings.HasPrefix(string(got), terrastack.GeneratedCodeHeader) {
		t.Fatal("generated code missing header")
	}

	parser := hcl.NewParser()
	_, err := parser.ParseBody(got, terrastack.GeneratedTfFilename)

	assert.NoError(t, err)
	// TODO: test parsed body
}

func versionAttribute() string {
	return "required_version " + terrastack.DefaultVersionConstraint()
}
