// Copyright 2022 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package e2etest

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
	t        *testing.T
	chdir    string
	loglevel string
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

func newCLIWithLogLevel(t *testing.T, chdir string, loglevel string) tscli {
	return tscli{
		t:        t,
		chdir:    chdir,
		loglevel: loglevel,
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

	loglevel := ts.loglevel
	if loglevel == "" {
		loglevel = "fatal"
	}

	if len(args) > 0 { // Avoid failing test when calling terramate with no args
		allargs = append(allargs, "--log-level", loglevel)
		allargs = append(allargs, "--log-fmt", "text")
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

	assertRunResult(t, got, runExpected{})
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
		if want.StdoutRegex != "" {
			matched, err := regexp.MatchString(want.StdoutRegex, stdout)
			assert.NoError(t, err, "failed to compile regex %q", want.StdoutRegex)

			if !matched {
				t.Errorf("%q stdout=\"%s\" does not match regex %q", got.Cmd,
					stdout,
					want.StdoutRegex,
				)
			}
		} else {
			assert.EqualStrings(t, wantStdout, stdout, "stdout mismatch")
		}
	}

	if !want.IgnoreStderr {
		if want.StderrRegex != "" {
			matched, err := regexp.MatchString(want.StderrRegex, got.Stderr)
			assert.NoError(t, err, "failed to compile regex %q", want.StderrRegex)

			if !matched {
				t.Errorf("%q stderr=\"%s\" does not match regex %q", got.Cmd,
					got.Stderr,
					want.StderrRegex,
				)
			}
		} else {
			assert.EqualStrings(t, want.Stderr, got.Stderr, "stderr mismatch")
		}
	}

	assert.EqualInts(t, want.Status, got.Status, "exit status mismatch")
}
