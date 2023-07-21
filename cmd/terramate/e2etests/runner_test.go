// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package e2etest

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/google/go-cmp/cmp"
	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/test"
)

const defaultErrExitStatus = 1

const testCliConfigFormat = `
user_terramate_dir = "%s"
`

type tmcli struct {
	t         *testing.T
	chdir     string
	loglevel  string
	env       []string
	appendEnv []string
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

// buffer provides a concurrency safe implementation of a bytes.Buffer
// It is not safe to copy the buffer.
type buffer struct {
	b bytes.Buffer
	m sync.Mutex
}

func (b *buffer) Read(p []byte) (n int, err error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.Read(p)
}

func (b *buffer) Write(p []byte) (n int, err error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.Write(p)
}

func (b *buffer) String() string {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.String()
}

type testCmd struct {
	t      *testing.T
	cmd    *exec.Cmd
	stdin  *buffer
	stdout *buffer
	stderr *buffer
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

func (tc *testCmd) exitCode() int {
	return tc.cmd.ProcessState.ExitCode()
}

// newCmd creates a new terramate command prepared to executed.
func (tm tmcli) newCmd(args ...string) *testCmd {
	t := tm.t
	t.Helper()

	stdin := &buffer{}
	stdout := &buffer{}
	stderr := &buffer{}

	allargs := []string{}
	if tm.chdir != "" {
		allargs = append(allargs, "--chdir", tm.chdir)
	}

	loglevel := tm.loglevel
	if loglevel == "" {
		loglevel = "error"
	}

	if len(args) > 0 { // Avoid failing test when calling terramate with no args
		allargs = append(allargs, "--log-level", loglevel)
		allargs = append(allargs, "--log-fmt", "text")
	}

	allargs = append(allargs, args...)

	env := tm.env
	if len(env) == 0 {
		env = os.Environ()
	}
	env = append(env, "CHECKPOINT_DISABLE=1")

	// custom cliconfig file
	userTmpDir := t.TempDir()
	cliConfigPath := test.WriteFile(t, userTmpDir, "terramate.rc", fmt.Sprintf(testCliConfigFormat, strings.Replace(userTmpDir, "\\", "\\\\", -1)))
	env = append(env,
		"TM_CLI_CONFIG_FILE="+cliConfigPath,
		"ACTIONS_ID_TOKEN_REQUEST_URL=",
		"ACTIONS_ID_TOKEN_REQUEST_TOKEN=",
	)

	env = append(env, tm.appendEnv...)

	// fake credentials
	type MyCustomClaims struct {
		Email string `json:"email"`
		jwt.StandardClaims
	}

	claims := MyCustomClaims{
		"batman@example.com",
		jwt.StandardClaims{
			ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
			Issuer:    "terramate-tests",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	fakeJwt, err := token.SignedString([]byte("test"))
	assert.NoError(t, err)
	test.WriteFile(t, userTmpDir, "credentials.tmrc.json", fmt.Sprintf(`{"id_token": "%s", "refresh_token": "abcd"}`, fakeJwt))

	cmd := exec.Command(terramateTestBin, allargs...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = stdin
	cmd.Env = env

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

func (tm tmcli) stacksRunOrder(args ...string) runResult {
	return tm.run(append([]string{"experimental", "run-order"}, args...)...)
}

func (tm tmcli) stacksRunGraph(args ...string) runResult {
	return tm.run(append([]string{"experimental", "run-graph"}, args...)...)
}

func (tm tmcli) listStacks(args ...string) runResult {
	return tm.run(append([]string{"list"}, args...)...)
}

func (tm tmcli) triggerStack(stack string) runResult {
	return tm.run([]string{"experimental", "trigger", stack}...)
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
			if diff := cmp.Diff(wantStdout, stdout); diff != "" {
				t.Errorf("stdout mismatch (-want +got): %s", diff)
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
