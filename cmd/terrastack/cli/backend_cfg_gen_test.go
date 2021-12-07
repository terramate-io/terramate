package cli_test

import (
	"strings"
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

	got := stack.ReadGeneratedTf()

	if !strings.HasPrefix(got, terrastack.GeneratedCodeHeader) {
		t.Fatal("generated code missing header")
	}

	// Parse + test actual generated code
}

func (ts tscli) generate() runResult {
	return ts.run("generate", "--basedir", ts.wd)
}

func versionAttribute() string {
	return "required_version " + terrastack.DefaultVersionConstraint()
}
