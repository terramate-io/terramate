package cli_test

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

type tscli struct {
	t     *testing.T
	chdir string
}

type runResult struct {
	Cmd           string
	Stdout        string
	FlattenStdout bool
	IgnoreStdout  bool
	Stderr        string
	IgnoreStderr  bool
	Error         error
}

func newCLI(t *testing.T, chdir string) tscli {
	return tscli{
		t:     t,
		chdir: chdir,
	}
}

func (ts tscli) run(args ...string) runResult {
	t := ts.t
	t.Helper()

	stdin := &bytes.Buffer{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	allargs := []string{}
	if ts.chdir != "" {
		allargs = append(allargs, "--chdir", ts.chdir)
	}

	allargs = append(allargs, args...)

	cmd := exec.Command(terramateTestBin, allargs...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = stdin

	err := cmd.Run()
	return runResult{
		Cmd:    strings.Join(args, " "),
		Stdout: stdout.String(),
		Stderr: stderr.String(),
		Error:  err,
	}
}

func assertRun(t *testing.T, got runResult) {
	t.Helper()

	assertRunResult(t, got, runResult{IgnoreStdout: true, IgnoreStderr: true})
}

func assertRunResult(t *testing.T, got runResult, want runResult) {
	t.Helper()

	stdout := got.Stdout
	wantStdout := want.Stdout
	if want.FlattenStdout {
		stdout = flatten(stdout)
		wantStdout = flatten(wantStdout)
	}

	if !want.IgnoreStdout && stdout != wantStdout {
		t.Errorf("%q stdout=\"%s\" != wanted=\"%s\"", got.Cmd, stdout, wantStdout)
	}

	if !want.IgnoreStderr && got.Stderr != want.Stderr {
		t.Errorf("%q stderr=\"%s\" != wanted=\"%s\"", got.Cmd, got.Stderr, want.Stderr)
	}

	if !errors.Is(got.Error, want.Error) {
		t.Errorf("%q got.Error=[%v] != want.Error=[%v]", got.Cmd, got.Error, want.Error)
	}
}
