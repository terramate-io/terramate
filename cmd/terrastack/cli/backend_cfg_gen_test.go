package cli_test

import (
	"testing"

	"github.com/mineiros-io/terrastack"
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
	assertRunResult(t, ts.generate(), runResult{IgnoreStdout: true})

	// TODO(katcipis): implement actual generation
	//want := fmt.Sprintf(`%s
	//terraform {
	//%s
	//}`, terrastack.GeneratedCodeHeader, backendBlock)
	//got := stack.ReadGeneratedTf()

	//assert.EqualStrings(t, want, got, "generated terraform file mismatch")
}

func (ts tscli) generate() runResult {
	return ts.run("generate", "--basedir", ts.wd)
}

func versionAttribute() string {
	return "required_version " + terrastack.DefaultVersionConstraint()
}
