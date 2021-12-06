package cli_test

import (
	"fmt"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terrastack"
	"github.com/mineiros-io/terrastack/test/sandbox"
)

func TestBackendConfigOnLeafSingleStack(t *testing.T) {
	t.Skip("TODO: failing for now, yay for tests first =P")

	s := sandbox.New(t)
	stack := s.CreateStack("stack")

	backendBlock := `backend "type" {
    param = "value"
}`

	stack.CreateConfig(`terrastack {
  required_version %s
  %s
}`, terrastack.DefaultVersionConstraint(), backendBlock)

	cli := newCLI(t, s.BaseDir())
	gen := cli.run("generate", s.BaseDir())
	assertRunResult(t, gen, runResult{IgnoreStdout: true})

	want := fmt.Sprintf(`%s
terraform {
	%s
}`, terrastack.GeneratedCodeHeader, backendBlock)
	got := stack.ReadGeneratedTf()

	assert.EqualStrings(t, want, got, "generated terraform file mismatch")
}
