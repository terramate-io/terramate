// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package e2etest

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
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
	environ   []string
	appendEnv []string

	userDir string
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

func newCLI(t *testing.T, chdir string, env ...string) tmcli {
	tm := tmcli{
		t:     t,
		chdir: chdir,
	}
	if len(env) == 0 {
		env = os.Environ()
	}
	env = append(env, "CHECKPOINT_DISABLE=1")
	// custom cliconfig file
	tm.userDir = t.TempDir()
	cliConfigPath := test.WriteFile(t, tm.userDir, "terramate.rc", fmt.Sprintf(testCliConfigFormat, strings.Replace(tm.userDir, "\\", "\\\\", -1)))
	env = append(env,
		"TM_CLI_CONFIG_FILE="+cliConfigPath,
		"ACTIONS_ID_TOKEN_REQUEST_URL=",
		"ACTIONS_ID_TOKEN_REQUEST_TOKEN=",
	)
	tm.environ = env
	return tm
}

func newCLIWithLogLevel(t *testing.T, chdir string, loglevel string) tmcli {
	tm := newCLI(t, chdir)
	tm.loglevel = loglevel
	return tm
}

func (tm *tmcli) prependToPath(dir string) {
	envKeyEquality := func(s1, s2 string) bool { return s1 == s2 }
	if runtime.GOOS == "windows" {
		envKeyEquality = strings.EqualFold
	}
	addTo := func(env []string, dir string) bool {
		for i, v := range env {
			eqPos := strings.Index(v, "=")
			key := v[:eqPos]
			oldv := v[eqPos+1:]
			if envKeyEquality(key, "PATH") {
				v = key + "=" + dir + string(os.PathListSeparator) + oldv
				env[i] = v
				return true
			}
		}
		return false
	}

	found := addTo(tm.appendEnv, dir)
	if !found && !addTo(tm.environ, dir) {
		tm.appendEnv = append(tm.appendEnv, fmt.Sprintf("PATH=%s", dir))
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
	env := append(tm.environ, tm.appendEnv...)

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
	test.WriteFile(t, tm.userDir, "credentials.tmrc.json", fmt.Sprintf(`{"id_token": "%s", "refresh_token": "abcd"}`, fakeJwt))

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

type runMode int

const (
	normalRun runMode = iota
	hangRun
	sleepRun
)

type runFixture struct {
	rootdir string
	flags   []string
	mode    runMode

	cli *tmcli
	cmd []string
}

const hangTimeout = time.Minute

// newRunFixture returns a new runFixture ready to be executed which supports 3 execution modes:
//   - normalRun: execute any command and wait for its completion.
//   - hangRun:   execute an specialized command which never terminates and ignore all signals.
//   - sleepRun:  execute an specialized command which sleeps for a configured amount of seconds.
//
// In the case of [hangRun] mode, the steps below will happen:
//  1. The helper `hang` command is invoked.
//  2. The test will poll the stdout buffer waiting for a "ready" message.
//  3. When the process is ready, the testing will send CTRL-C twice and wait for the process acknowledge.
//  4. The test will wait for [hangTimeout] seconds for a graceful exit, otherwise the process is killed with SIGKILL.
//
// In the case of [sleepRun] mode, the steps below will happen:
//  1. The helper `sleep` command is invoked.
//  2. The test will poll the stdout buffer waiting for a "ready" message.
//  3. When the process is ready, the testing will send a single CTRL-C.
//  4. The test will wait for process graceful exit.
//
// Usage:
//
//	cli := newCli(t, chdir)
//	fixture := newRunFixture(hangRun, s.RootDir(), "--cloud-sync-deployment")
//	result = fixture.run()
//	assertRunResult(t, result, expected)
func (tm *tmcli) newRunFixture(mode runMode, rootdir string, flags ...string) runFixture {
	return runFixture{
		rootdir: rootdir,
		cli:     tm,
		flags:   flags,
		mode:    mode,
	}
}

func (run *runFixture) run() runResult {
	t := run.cli.t
	t.Helper()

	args := []string{"run"}
	args = append(args, run.flags...)
	args = append(args, "--")

	switch run.mode {
	case normalRun:
		cmd := run.cmd
		if len(cmd) == 0 {
			// in normalMode the user can override the invoked command.
			cmd = append(cmd, testHelperBin, "stack-abs-path", run.rootdir)
		}
		args = append(args, cmd...)
	case hangRun:
		args = append(args, testHelperBin, "hang")
	case sleepRun:
		args = append(args, testHelperBin, "sleep", "1m")
	default:
		t.Fatalf("unexpected run mode: %d", run.mode)
	}

	exec := run.cli.newCmd(args...)

	switch run.mode {
	case normalRun:
		exec.start()
		_ = exec.wait()
	case hangRun:
		testHangProcess(t, exec)
	case sleepRun:
		testSleepProcess(t, exec)
	}

	return runResult{
		Cmd:    strings.Join(args, " "),
		Stdout: exec.stdout.String(),
		Stderr: exec.stderr.String(),
		Status: exec.exitCode(),
	}
}

func testHangProcess(t *testing.T, exec *testCmd) {
	exec.setpgid()
	exec.start()
	errs := make(chan error)
	go func() {
		errs <- exec.wait()
		close(errs)
	}()
	assert.NoError(t, pollBufferForMsgs(exec.stdout, errs, "ready"))
	sendUntilMsgIsReceived(t, exec, os.Interrupt, "ready", "interrupt")
	sendUntilMsgIsReceived(t, exec, os.Interrupt, "ready", "interrupt", "interrupt")

	// We can't check the last interrupt message since the child process
	// may be killed by Terramate with SIGKILL before it gets the signal
	// or it is able to send messages to stdout.
	ctx, cancel := context.WithTimeout(context.Background(), hangTimeout)
	defer cancel()

outer:
	for ctx.Err() == nil {
		t.Log("sending last interrupt signal to terramate")
		exec.signalGroup(os.Interrupt)

		sendctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		select {
		case err := <-errs:
			t.Logf("terramate err: %v", err)
			t.Logf("terramate stdout:\n%s\n", exec.stdout.String())
			t.Logf("terramate stderr:\n%s\n", exec.stderr.String())
			assert.Error(t, err)
			break outer
		case <-sendctx.Done():
			t.Log("terramate still running, re-sending interrupt")
		}
	}

	t.Logf("terramate stdout:\n%s\n", exec.stdout.String())
	t.Logf("terramate stderr:\n%s\n", exec.stderr.String())
}

func testSleepProcess(t *testing.T, exec *testCmd) {
	exec.setpgid()
	exec.start()
	done := make(chan error)
	go func() {
		done <- exec.wait()
	}()
	assert.NoError(t, pollBufferForMsgs(exec.stdout, done, "ready"))
	exec.signalGroup(os.Interrupt)
	<-done
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
