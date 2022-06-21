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
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
)

const defaultErrExitStatus = 1

type tmcli struct {
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

func newCLI(t *testing.T, chdir string) tmcli {
	return tmcli{
		t:     t,
		chdir: chdir,
	}
}

func newCLIWithLogLevel(t *testing.T, chdir string, loglevel string) tmcli {
	return tmcli{
		t:        t,
		chdir:    chdir,
		loglevel: loglevel,
	}
}

type testCmd struct {
	t      *testing.T
	cmd    *exec.Cmd
	stdin  *bytes.Buffer
	stdout *bytes.Buffer
	stderr *bytes.Buffer
}

func (tc *testCmd) run() error {
	return tc.cmd.Run()
}

func (tc *testCmd) start() {
	t := tc.t
	t.Helper()

	assert.NoError(t, tc.cmd.Start())
}

func (tc *testCmd) wait() error {
	return tc.cmd.Wait()
}

func (tc *testCmd) signal(s os.Signal) {
	t := tc.t
	t.Helper()

	err := tc.cmd.Process.Signal(s)
	assert.NoError(t, err)
}

func (tc *testCmd) exitCode() int {
	return tc.cmd.ProcessState.ExitCode()
}

// newCmd creates a new terramate command prepared to executed.
func (tm tmcli) newCmd(args ...string) *testCmd {
	t := tm.t
	t.Helper()

	stdin := &bytes.Buffer{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	allargs := []string{}
	if tm.chdir != "" {
		allargs = append(allargs, "--chdir", tm.chdir)
	}

	loglevel := tm.loglevel
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

	return &testCmd{
		t:      t,
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
	}
}

func (tm tmcli) run(args ...string) runResult {
	t := tm.t
	t.Helper()

	cmd := tm.newCmd(args...)
	_ = cmd.run()

	return runResult{
		Cmd:    strings.Join(args, " "),
		Stdout: cmd.stdout.String(),
		Stderr: cmd.stderr.String(),
		Status: cmd.exitCode(),
	}
}

func (tm tmcli) initStack(args ...string) runResult {
	return tm.run(append([]string{"experimental", "init-stack"}, args...)...)
}

func (tm tmcli) stacksRunOrder(args ...string) runResult {
	return tm.run(append([]string{"experimental", "run-order"}, args...)...)
}

func (tm tmcli) stacksRunGraph(args ...string) runResult {
	return tm.run(append([]string{"experimental", "run-graph"}, args...)...)
}

func (tm tmcli) listStacks(args ...string) runResult {
	return tm.run(append([]string{"list"}, args...)...)
}

func (tm tmcli) listChangedStacks(args ...string) runResult {
	return tm.listStacks(append([]string{"--changed"}, args...)...)
}

func assertRun(t *testing.T, got runResult) {
	t.Helper()

	assertRunResult(t, got, runExpected{})
}

func assertRunResult(t *testing.T, got runResult, want runExpected) {
	t.Helper()

	// Why not use assert functions here but use t.Error ? We get simple errors like:
	// "wanted[stack] but got[].stdout mismatch"
	// And nothing else.
	// In case of errors, more detailed information on the errors
	// like what we got on stderr helps the dev to understand
	// in more detail why it has failed.

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
			if wantStdout != stdout {
				t.Errorf("stdout mismatch: got %q != want %q", stdout, wantStdout)
			}
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
			if want.Stderr != got.Stderr {
				t.Errorf("stderr mismatch: got %q != want %q", got.Stderr, want.Stderr)
			}
		}
	}

	assert.EqualInts(t, want.Status, got.Status, "exit status mismatch")
}
