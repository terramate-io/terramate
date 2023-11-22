// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build !darwin

package core_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/madlambda/spells/assert"
	. "github.com/terramate-io/terramate/cmd/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestRunSendsSigkillIfSIGINTx3(t *testing.T) {
	t.Parallel()

	t.Run("SIGINT (x3) --continue-on-error=false", func(t *testing.T) {
		testSIGINTx3(t, false)
	})

	t.Run("SIGINT (x3) --continue-on-error=true", func(t *testing.T) {
		testSIGINTx3(t, true)
	})
}

func TestRunAbortNextStacksIfSIGINTx1(t *testing.T) {
	t.Parallel()

	t.Run("SIGINT (x1) --continue-on-error=false", func(t *testing.T) {
		testSIGINTx1(t, false)
	})

	t.Run("SIGINT (x1) --continue-on-error=true", func(t *testing.T) {
		testSIGINTx1(t, true)
	})
}

func testSIGINTx3(t *testing.T, continueOnError bool) {
	t.Parallel()
	s := sandbox.New(t)
	s.BuildTree([]string{
		// multiple stacks so we can check execution is aborted.
		`s:stack1`,
		`s:stack2`,
		`s:stack3`,
	})

	git := s.Git()
	git.Add(".")
	git.CommitAll("first commit")

	continueOnErrorFlag := `--continue-on-error=`
	if continueOnError {
		continueOnErrorFlag += `true`
	} else {
		continueOnErrorFlag += `false`
	}

	tm := NewCLI(t, s.RootDir())

	// Why the use of HEREDOC?
	// We need conditionally execute a different command depending on the stack,
	// then the most portable way is using `terramate --eval`.
	// The problem is that then all parts of the command is evaluated but the
	// first string is the program path and Windows uses \ (backslash)
	// as path separator and this makes an invalid HCL string as \ is used for
	// escaping symbols or unicode code sequences.
	// The HEREDOC is used to create a raw string with no characters interpretation.
	cmd := tm.NewCmd("run", continueOnErrorFlag, "--eval", HelperPathAsHCL,
		// this would be simplified by supporting list in the evaluation output.
		// NOTE: an extra parameter is provided for `hang` but it's ignore by the helper.
		`${terramate.stack.path.absolute == "/stack2" ? "hang" : "echo"}`,
		`${terramate.stack.path.absolute == "/stack2" ? "" : terramate.stack.path.absolute}`,
	)
	// To simulate something similar to a terminal we run
	// terramate in a separate pgid here and then send a signal to
	// the whole group. The test process must not be part of this group.
	cmd.Setpgid()
	cmd.Start()

	expectedStdout := []string{
		"/stack1", "ready",
	}

	errs := make(chan error)
	go func() {
		errs <- cmd.Wait()
		close(errs)
	}()

	assert.NoError(
		t,
		PollBufferForMsgs(cmd.Stdout, errs, expectedStdout...),
		"failed to start: %s", cmd.Stderr.String(),
	)

	expectedStdout = append(expectedStdout, "interrupt")
	SendUntilMsgIsReceived(t, cmd, os.Interrupt, expectedStdout...)

	expectedStdout = append(expectedStdout, "interrupt")
	SendUntilMsgIsReceived(t, cmd, os.Interrupt, expectedStdout...)

	// We can't check the last interrupt message since the child process
	// may be killed by Terramate with SIGKILL before it gets the signal
	// or it is able to send messages to stdout.
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	for ctx.Err() == nil {
		t.Log("sending last interrupt signal to terramate")
		cmd.SignalGroup(os.Interrupt)

		sendctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		select {
		case err := <-errs:
			stdoutStr := cmd.Stdout.String()
			stderrStr := cmd.Stderr.String()
			t.Logf("terramate err: %v", err)
			t.Logf("terramate stdout:\n%s\n", stdoutStr)
			t.Logf("terramate stderr:\n%s\n", stderrStr)
			assert.Error(t, err)
			if strings.Contains(stdoutStr, `/stack3`) {
				t.Fatalf("subsequent stacks not canceled")
			}
			return
		case <-sendctx.Done():
			t.Log("terramate still running, re-sending interrupt")
		}
	}

	t.Error("waiting for terramate to exit for too long")
	t.Logf("terramate stdout:\n%s\n", cmd.Stdout.String())
	t.Logf("terramate stderr:\n%s\n", cmd.Stderr.String())
}

func testSIGINTx1(t *testing.T, continueOnError bool) {
	t.Parallel()
	s := sandbox.New(t)
	s.BuildTree([]string{
		`s:stack1`,
		`s:stack2`,
		`s:stack3`,
		`s:stack4`,
		`s:stack5`,
	})

	git := s.Git()
	git.Add(".")
	git.CommitAll("first commit")

	continueOnErrorFlag := `--continue-on-error=`
	if continueOnError {
		continueOnErrorFlag += `true`
	} else {
		continueOnErrorFlag += `false`
	}

	tm := NewCLI(t, s.RootDir())
	cmd := tm.NewCmd("run", continueOnErrorFlag, "--eval", HelperPathAsHCL,
		// this would be simplified by supporting list in the evaluation output.
		`${terramate.stack.path.absolute == "/stack2" ? "sleep" : "echo"}`,
		// A big sleep time because MacOS runner in GHA is very slow.
		// The test will not going to wait this whole time because eventually
		// MacOS will deliver the SIGINT and the process will abort.
		`${terramate.stack.path.absolute == "/stack2" ? "360s" : terramate.stack.path.absolute}`,
	)
	// To simulate something similar to a terminal we run
	// terramate in a separate pgid here and then send a signal to
	// the whole group. The test process must not be part of this group.
	cmd.Setpgid()
	cmd.Start()

	errs := make(chan error)
	go func() {
		errs <- cmd.Wait()
		close(errs)
	}()

	expectedStdout := []string{
		"/stack1", "ready",
	}

	assert.NoError(t, PollBufferForMsgs(cmd.Stdout, errs, expectedStdout...), cmd.Stderr.String())

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	for ctx.Err() == nil {
		t.Log("sending interrupt signal to terramate")
		cmd.SignalGroup(os.Interrupt)

		sendctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		select {
		case err := <-errs:
			stdoutStr := cmd.Stdout.String()
			stderrStr := cmd.Stderr.String()
			t.Logf("terramate err: %v", err)
			t.Logf("terramate stdout:\n%s\n", stdoutStr)
			t.Logf("terramate stderr:\n%s\n", stderrStr)
			assert.Error(t, err)
			if strings.Contains(stdoutStr, `stack5`) {
				t.Fatalf("subsequent stacks not canceled")
			}
			return
		case <-sendctx.Done():
			t.Log("terramate still running, re-sending interrupt")
		}
	}

	t.Error("waiting for terramate to exit for too long")
	t.Logf("terramate stdout:\n%s\n", cmd.Stdout.String())
	t.Logf("terramate stderr:\n%s\n", cmd.Stderr.String())
}
