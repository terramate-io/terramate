package cli_test

import (
	"os/exec"
	"testing"

	"github.com/mineiros-io/terrastack/test"
)

type TestEnv struct {
	t       *testing.T
	basedir string
}

// NewTestEnv creates a new test env, including a new
// temporary repository. All commands run using the test
// env will use this tmp dir as the working dir.
//
// It is a programming error to use a test env created
// with a *testing.T other than the one of the test
// using the test env, for a new test/sub-test always create
// a new test env for it.
func NewTestEnv(t *testing.T) *TestEnv {
	t.Helper()
	return &TestEnv{
		t:       t,
		basedir: test.TempDir(t, ""),
	}
}

// Cleanup will release any resources, like files, created
// by the test env, it is a programming error to use the test
// env after calling this method.
func (te *TestEnv) Cleanup() {
	te.t.Helper()
	test.RemoveAll(te.t, te.basedir)
}

// Run will run the given cmd with the provided args using
// the test env base dir as the command working dir.
// This method fails the test if the command fails, where
// a command failed is defined by its status code (!= 0).
func (te *TestEnv) Run(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = te.basedir

	if out, err := cmd.CombinedOutput(); err != nil {
		te.t.Errorf("failed to run: '%v' err: '%v' output: '%s'", cmd, err, string(out))
	}
}
