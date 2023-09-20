// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build !darwin

package e2etest

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/madlambda/spells/assert"
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

	tm := newCLI(t, s.RootDir())

	// Why the use of HEREDOC?
	// We need conditionally execute a different command depending on the stack,
	// then the most portable way is using `terramate --eval`.
	// The problem is that then all parts of the command is evaluated but the
	// first string is the program path and Windows uses \ (backslash)
	// as path separator and this makes an invalid HCL string as \ is used for
	// escaping symbols or unicode code sequences.
	// The HEREDOC is used to create a raw string with no characters interpretation.
	cmd := tm.newCmd("run", continueOnErrorFlag, "--eval", testHelperBinAsHCL,
		// this would be simplified by supporting list in the evaluation output.
		// NOTE: an extra parameter is provided for `hang` but it's ignore by the helper.
		`${terramate.stack.path.absolute == "/stack2" ? "hang" : "echo"}`,
		`${terramate.stack.path.absolute == "/stack2" ? "" : terramate.stack.path.absolute}`,
	)
	// To simulate something similar to a terminal we run
	// terramate in a separate pgid here and then send a signal to
	// the whole group. The test process must not be part of this group.
	cmd.setpgid()
	cmd.start()

	expectedStdout := []string{
		"/stack1", "ready",
	}

	errs := make(chan error)
	go func() {
		errs <- cmd.wait()
		close(errs)
	}()

	assert.NoError(
		t,
		pollBufferForMsgs(cmd.stdout, errs, expectedStdout...),
		"failed to start: %s", cmd.stderr.String(),
	)

	expectedStdout = append(expectedStdout, "interrupt")
	sendUntilMsgIsReceived(t, cmd, os.Interrupt, expectedStdout...)

	expectedStdout = append(expectedStdout, "interrupt")
	sendUntilMsgIsReceived(t, cmd, os.Interrupt, expectedStdout...)

	// We can't check the last interrupt message since the child process
	// may be killed by Terramate with SIGKILL before it gets the signal
	// or it is able to send messages to stdout.
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	for ctx.Err() == nil {
		t.Log("sending last interrupt signal to terramate")
		cmd.signalGroup(os.Interrupt)

		sendctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		select {
		case err := <-errs:
			stdoutStr := cmd.stdout.String()
			stderrStr := cmd.stderr.String()
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
	t.Logf("terramate stdout:\n%s\n", cmd.stdout.String())
	t.Logf("terramate stderr:\n%s\n", cmd.stderr.String())
}

func testSIGINTx1(t *testing.T, continueOnError bool) {
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

	tm := newCLI(t, s.RootDir())
	cmd := tm.newCmd("run", continueOnErrorFlag, "--eval", testHelperBinAsHCL,
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
	cmd.setpgid()
	cmd.start()

	errs := make(chan error)
	go func() {
		errs <- cmd.wait()
		close(errs)
	}()

	expectedStdout := []string{
		"/stack1", "ready",
	}

	assert.NoError(t, pollBufferForMsgs(cmd.stdout, errs, expectedStdout...), cmd.stderr.String())

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	for ctx.Err() == nil {
		t.Log("sending interrupt signal to terramate")
		cmd.signalGroup(os.Interrupt)

		sendctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		select {
		case err := <-errs:
			stdoutStr := cmd.stdout.String()
			stderrStr := cmd.stderr.String()
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
	t.Logf("terramate stdout:\n%s\n", cmd.stdout.String())
	t.Logf("terramate stderr:\n%s\n", cmd.stderr.String())
}

// pollBufferForMsgs will check if each message is present on the buffer
// on signal handling sometimes we get extraneous signals on the test process
// like "urgent I/O condition". This function will ignore any unknown messages
// in between but check that at least all msgs where received in the provided
// order (but ignoring unknown messages in between).
func pollBufferForMsgs(buf *buffer, done chan error, wantMsgs ...string) error {
	const (
		timeout      = 10 * time.Second
		pollInterval = 30 * time.Millisecond
	)

	var elapsed time.Duration

	for {
		select {
		case err := <-done:
			return err
		default:
			gotMsgs := strings.Split(buf.String(), "\n")
			wantIndex := 0

			for _, got := range gotMsgs {
				if got == wantMsgs[wantIndex] {
					wantIndex++
				}

				if wantIndex == len(wantMsgs) {
					return nil
				}
			}

			time.Sleep(pollInterval)
			elapsed += pollInterval
			if elapsed > timeout {
				return fmt.Errorf("timeout polling: wanted: %v got: %v", wantMsgs, gotMsgs)
			}
		}
	}
}

func sendUntilMsgIsReceived(t *testing.T, cmd *testCmd, signal os.Signal, msgs ...string) {
	t.Helper()
	// For some reason in some environments, specially CI ones,
	// signals are not being delivered, so we retry sending the signal.
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	for {
		cmd.signalGroup(signal)
		err := pollBufferForMsgs(cmd.stdout, make(chan error), msgs...)
		if err == nil {
			return
		}
		if ctx.Err() != nil {
			t.Fatal(err)
		}
	}
}
