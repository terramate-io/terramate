package cli_test

import (
	"bytes"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
)

const defaultErrExitStatus = 1

type tscli struct {
	t     *testing.T
	chdir string
}

type runResult struct {
	Cmd    string
	Stdout string
	Stderr string
	Status int
}

type runExpected struct {
	Stdout      string
	Stderr      string
	StdoutRegex string
	StderrRegex string

	IgnoreStdout bool
	IgnoreStderr bool

	FlattenStdout bool
	Status        int
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

	_ = cmd.Run()

	return runResult{
		Cmd:    strings.Join(args, " "),
		Stdout: stdout.String(),
		Stderr: stderr.String(),
		Status: cmd.ProcessState.ExitCode(),
	}
}

func assertRun(t *testing.T, got runResult) {
	t.Helper()

	assertRunResult(t, got, runExpected{IgnoreStdout: true, IgnoreStderr: true})
}

func assertRunResult(t *testing.T, got runResult, want runExpected) {
	t.Helper()

	if !want.IgnoreStdout {
		stdout := got.Stdout
		wantStdout := want.Stdout
		if want.FlattenStdout {
			stdout = flatten(stdout)
			wantStdout = flatten(wantStdout)
		}
		if want.StdoutRegex == "" {
			assert.EqualStrings(t, wantStdout, stdout, "stdout mismatch")
		} else {
			matched, err := regexp.MatchString(want.StdoutRegex, stdout)
			assert.NoError(t, err, "failed to compile regex %q", want.StdoutRegex)

			if !matched {
				t.Errorf("%q stdout=\"%s\" does not match regex %q", got.Cmd,
					stdout,
					want.StdoutRegex,
				)
			}
		}
	}

	if !want.IgnoreStderr {
		if want.StderrRegex == "" {
			assert.EqualStrings(t, want.Stderr, got.Stderr, "stderr mismatch")
		} else {
			matched, err := regexp.MatchString(want.StderrRegex, got.Stderr)
			assert.NoError(t, err, "failed to compile regex %q", want.StderrRegex)

			if !matched {
				t.Errorf("%q stderr=\"%s\" does not match regex %q", got.Cmd,
					got.Stderr,
					want.StderrRegex,
				)
			}
		}
	}

	assert.EqualInts(t, want.Status, got.Status, "exit status mismatch")
}
