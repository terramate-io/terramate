// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package runner

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/google/go-cmp/cmp"
	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/stack/trigger"
	"github.com/terramate-io/terramate/test"
)

const testCliConfigFormat = `
user_terramate_dir = "%s"
`

type (
	// CLI is a Terramate CLI runner.
	CLI struct {
		t         *testing.T
		Chdir     string
		LogLevel  string
		environ   []string
		AppendEnv []string

		wd      string
		userDir string
	}

	// RunResult specify the result of executing the cli.
	RunResult struct {
		Cmd    string
		Stdout string
		Stderr string
		Status int
	}

	// RunExpected specifies the expected result for the CLI execution.
	RunExpected struct {
		Stdout      string
		Stderr      string
		StdoutRegex string
		StderrRegex string

		StdoutRegexes []string
		StderrRegexes []string

		NoStdoutRegex   string
		NoStdoutRegexes []string

		NoStderrRegex   string
		NoStderrRegexes []string

		IgnoreStdout bool
		IgnoreStderr bool

		FlattenStdout bool
		Status        int
	}
)

// NewCLI creates a new runner for the CLI.
func NewCLI(t *testing.T, chdir string, env ...string) CLI {
	if toolsetTestPath == "" {
		panic("runner is not initialized: use runner.Setup()")
	}
	tm := CLI{
		t:     t,
		Chdir: chdir,
	}
	if len(env) == 0 {
		// by default, it's assumed human mode
		env = RemoveEnv(os.Environ(), "CI", "GITHUB_ACTIONS", "GITHUB_TOKEN")
	}
	// environments below are never used in automation.
	env = RemoveEnv(env, "TMC_API_HOST", "TMC_API_IDP_KEY")
	// this needs to be deleted from environment otherwise CLI GHA tests will try to issue JWT tokens.
	env = RemoveEnv(env, "ACTIONS_ID_TOKEN_REQUEST_URL", "ACTIONS_ID_TOKEN_REQUEST_TOKEN")
	// this must be always disabled otherwise we contaminate the telemetry endpoint.
	env = append(env, "CHECKPOINT_DISABLE=1")
	// sanity check for cases where user has this configured in their environment.
	if index := slices.Index(env, "TMC_API_URL"); index >= 0 {
		apiURL := env[index]
		if !strings.HasPrefix(apiURL, "http://") {
			panic("tests are picking up the wrong API URL")
		}
	}
	// custom cliconfig file
	tm.userDir = test.TempDir(t)
	cliConfigPath := test.WriteFile(t, tm.userDir, "terramate.rc", fmt.Sprintf(testCliConfigFormat, strings.Replace(tm.userDir, "\\", "\\\\", -1)))
	env = append(env,
		"TM_CLI_CONFIG_FILE="+cliConfigPath,
	)
	tm.environ = env
	return tm
}

// NewInteropCLI creates a new runner CLI suited for interop tests.
func NewInteropCLI(t *testing.T, chdir string, env ...string) CLI {
	if toolsetTestPath == "" {
		panic("runner is not initialized: use runner.Setup()")
	}
	tm := CLI{
		t:     t,
		Chdir: chdir,
	}
	if len(env) == 0 {
		env = os.Environ()
	}
	env = append(env, "CHECKPOINT_DISABLE=1")
	tm.environ = env
	return tm
}

// SetWorkingDir sets the process working directory. Not to be confused with tm.Chdir
// which controls the `--chdir` flag.
func (tm *CLI) SetWorkingDir(dir string) {
	tm.wd = dir
}

// PrependToPath prepend the provided directory to the OS's PATH
// environment variable in a portable way.
func (tm *CLI) PrependToPath(dir string) {
	var found bool
	tm.AppendEnv, found = test.PrependToPath(tm.AppendEnv, dir)
	if !found {
		tm.environ, found = test.PrependToPath(tm.environ, dir)
		if !found {
			tm.AppendEnv = append(tm.AppendEnv, fmt.Sprintf("PATH=%s", dir))
		}
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

// Cmd is a generic runner that can be used to run any command.
type Cmd struct {
	t      *testing.T
	cmd    *exec.Cmd
	Stdin  *buffer
	Stdout *buffer
	Stderr *buffer
}

// Run the command.
func (tc *Cmd) Run() error {
	return tc.cmd.Run()
}

// ExitCode returns the exit code for a finished command.
func (tc *Cmd) ExitCode() int {
	return tc.cmd.ProcessState.ExitCode()
}

// NewCmd creates a new terramate command prepared to executed.
func (tm CLI) NewCmd(args ...string) *Cmd {
	t := tm.t
	t.Helper()

	stdin := &buffer{}
	stdout := &buffer{}
	stderr := &buffer{}

	allargs := []string{}
	if tm.Chdir != "" {
		allargs = append(allargs, "--chdir", tm.Chdir)
	}

	loglevel := tm.LogLevel
	if loglevel == "" {
		loglevel = "error"
	}

	if len(args) > 0 { // Avoid failing test when calling terramate with no args
		allargs = append(allargs, "--log-level", loglevel)
		allargs = append(allargs, "--log-fmt", "text")
	}

	allargs = append(allargs, args...)
	env := append(tm.environ, tm.AppendEnv...)

	// fake credentials
	type MyCustomClaims struct {
		Email string `json:"email"`
		jwt.StandardClaims
	}

	claims := MyCustomClaims{
		"batman@terramate.io",
		jwt.StandardClaims{
			ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
			Issuer:    "terramate-tests",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	fakeJwt, err := token.SignedString([]byte("test"))
	assert.NoError(t, err)
	test.WriteFile(t, tm.userDir, "credentials.tmrc.json", fmt.Sprintf(`{"id_token": "%s", "refresh_token": "abcd", "provider": "Google"}`, fakeJwt))

	cmd := exec.Command(tm.terramatePath(), allargs...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = stdin
	cmd.Env = env
	cmd.Dir = tm.wd

	return &Cmd{
		t:      t,
		cmd:    cmd,
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	}
}

func (tm CLI) terramatePath() string {
	return filepath.Join(toolsetTestPath, "terramate") + platExeSuffix()
}

// Run the cli command.
func (tm CLI) Run(args ...string) RunResult {
	t := tm.t
	t.Helper()

	cmd := tm.NewCmd(args...)
	_ = cmd.Run()

	return RunResult{
		Cmd:    strings.Join(args, " "),
		Stdout: cmd.Stdout.String(),
		Stderr: cmd.Stderr.String(),
		Status: cmd.ExitCode(),
	}
}

// RunWithStdin runs the CLI but uses the provided string as stdin.
func (tm CLI) RunWithStdin(stdin string, args ...string) RunResult {
	t := tm.t
	t.Helper()

	cmd := tm.NewCmd(args...)
	cmd.Stdin.b.WriteString(stdin)
	_ = cmd.Run()

	return RunResult{
		Cmd:    strings.Join(args, " "),
		Stdout: cmd.Stdout.String(),
		Stderr: cmd.Stderr.String(),
		Status: cmd.ExitCode(),
	}
}

// RunScript is a helper for executing `terramate run-script`.
func (tm CLI) RunScript(args ...string) RunResult {
	return tm.Run(append([]string{"script", "run"}, args...)...)
}

// StacksRunGraph is a helper for executing `terramate experimental run-graph`.
func (tm CLI) StacksRunGraph(args ...string) RunResult {
	return tm.Run(append([]string{"experimental", "run-graph"}, args...)...)
}

// ListStacks is a helper for executinh `terramate list`.
func (tm CLI) ListStacks(args ...string) RunResult {
	return tm.Run(append([]string{"list"}, args...)...)
}

// ListChangedStacks is a helper for executing `terramate list --changed`.
func (tm CLI) ListChangedStacks(args ...string) RunResult {
	return tm.ListStacks(append([]string{"--changed"}, args...)...)
}

func triggerKindFlag(kind trigger.Kind) string {
	var kindFlag string
	switch kind {
	case trigger.Changed:
		kindFlag = "--change"
	case trigger.Ignored:
		kindFlag = "--ignore-change"
	default:
		panic(fmt.Sprintf("unsupported trigger kind: %s", kind))
	}
	return kindFlag
}

// TriggerStack is a helper for executing `terramate experimental trigger`.
func (tm CLI) TriggerStack(kind trigger.Kind, stack string) RunResult {
	kindFlag := triggerKindFlag(kind)
	return tm.Run([]string{"experimental", "trigger", kindFlag, stack}...)
}

// TriggerRecursively is a helper for executing `terramate experimental trigger --recursively`.
func (tm CLI) TriggerRecursively(kind trigger.Kind, base string) RunResult {
	return tm.Trigger(triggerKindFlag(kind), "--recursive", base)
}

// Trigger is a helper for executing `terramate experimental trigger ...`.
func (tm CLI) Trigger(flags ...string) RunResult {
	args := []string{"experimental", "trigger"}
	args = append(args, flags...)
	return tm.Run(args...)
}

// AssertRun asserts that the provided run result is successfully and with no output.
func AssertRun(t *testing.T, got RunResult) {
	t.Helper()

	AssertRunResult(t, got, RunExpected{})
}

// AssertRunResult asserts that the result matches the expected.
func AssertRunResult(t *testing.T, got RunResult, want RunExpected) {
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
			want.StdoutRegexes = append(want.StdoutRegexes, want.StdoutRegex)
		}
		if want.NoStdoutRegex != "" {
			want.NoStdoutRegexes = append(want.NoStdoutRegexes, want.NoStdoutRegex)
		}
		if len(want.StdoutRegexes) > 0 || len(want.NoStdoutRegexes) > 0 {
			for _, stdoutRegex := range want.StdoutRegexes {
				matched, err := regexp.MatchString(stdoutRegex, stdout)
				assert.NoError(t, err, "failed to compile regex %q", stdoutRegex)

				if !matched {
					t.Errorf("%q stdout=\"%s\" does not match regex %q", got.Cmd,
						stdout,
						stdoutRegex,
					)
				}
			}

			for _, noStdoutRegex := range want.NoStdoutRegexes {
				matched, err := regexp.MatchString(noStdoutRegex, stdout)
				assert.NoError(t, err, "failed to compile regex %q", noStdoutRegex)

				if matched {
					t.Errorf("%q stdout=\"%s\" matches regex %q but it should not", got.Cmd,
						stdout,
						noStdoutRegex,
					)
				}
			}
		} else {
			if diff := cmp.Diff(wantStdout, stdout); diff != "" {
				t.Errorf("stdout mismatch (-want +got): %s", diff)
			}
		}
	}

	if !want.IgnoreStderr {
		if want.StderrRegex != "" {
			want.StderrRegexes = append(want.StderrRegexes, want.StderrRegex)
		}
		if want.NoStderrRegex != "" {
			want.NoStderrRegexes = append(want.NoStderrRegexes, want.NoStderrRegex)
		}

		if len(want.StderrRegexes) > 0 || len(want.NoStderrRegexes) > 0 {
			for _, stderrRegex := range want.StderrRegexes {
				matched, err := regexp.MatchString(stderrRegex, got.Stderr)
				assert.NoError(t, err, "failed to compile regex %q", stderrRegex)

				if !matched {
					t.Errorf("%q stderr=\"%s\" does not match regex %q", got.Cmd,
						got.Stderr,
						stderrRegex,
					)
				}
			}

			for _, noStderrRegex := range want.NoStderrRegexes {
				matched, err := regexp.MatchString(noStderrRegex, got.Stderr)
				assert.NoError(t, err, "failed to compile regex %q", noStderrRegex)

				if matched {
					t.Errorf("%q stderr=\"%s\" matches regex %q but it should not", got.Cmd,
						got.Stderr,
						noStderrRegex,
					)
				}
			}
		} else {
			if want.Stderr != got.Stderr {
				t.Errorf("stderr mismatch: got %q != want %q", got.Stderr, want.Stderr)
			}
		}
	}

	assert.EqualInts(t, want.Status, got.Status, "exit status mismatch")
}

// RemoveEnv removes an environment variable from the set.
func RemoveEnv(environ []string, names ...string) []string {
	ret := make([]string, 0, len(environ))
	for _, env := range environ {
		toBeDeleted := false
		for _, name := range names {
			if strings.HasPrefix(env, name+"=") {
				toBeDeleted = true
				break
			}
		}
		if !toBeDeleted {
			ret = append(ret, env)
		}
	}
	return ret
}

// remove tabs and newlines
func flatten(s string) string {
	return strings.Replace((strings.Replace(s, "\n", "", -1)), "\t", "", -1)
}
