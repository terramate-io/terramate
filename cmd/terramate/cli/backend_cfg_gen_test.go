package cli_test

import (
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestBackendConfigOnLeafSingleStack(t *testing.T) {
	s := sandbox.New(t)
	stack := s.CreateStack("stack")

	backendBlock := `backend "type" {
		param = "value"
	}`

	stack.CreateConfig(`terramate {
  %s
  %s
}`, versionAttribute(), backendBlock)

	ts := newCLI(t, s.BaseDir())
	assertRunResult(t, ts.run("generate"), runResult{IgnoreStdout: true})

	got := stack.ReadGeneratedTf()

	if !strings.HasPrefix(string(got), terramate.GeneratedCodeHeader) {
		t.Fatal("generated code missing header")
	}

	parser := hcl.NewParser()
	_, err := parser.ParseBody(got, terramate.GeneratedTfFilename)

	assert.NoError(t, err)
	// TODO: test parsed body
}

func versionAttribute() string {
	return "required_version " + terramate.DefaultVersionConstraint()
}
