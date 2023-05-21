// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

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

func TestRunSendsSigkillIfCmdIgnoresInterruptionSignals(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.CreateStack("stack-1")

	git := s.Git()
	git.Add(".")
	git.CommitAll("first commit")

	tm := newCLIWithLogLevel(t, s.RootDir(), "trace")
	cmd := tm.newCmd("run", testHelperBin, "hang")
	// To simulate something similar to a terminal we run
	// terramate in a separate pgid here and then send a signal to
	// the whole group. The test process must not be part of this group.
	cmd.setpgid()

	cmd.start()

	assert.NoError(t, pollBufferForMsgs(cmd.stdout, "ready"))

	// On rare occasions on macos we seen to lose some SIGINT's
	sendUntilMsgIsReceived(t, cmd, os.Interrupt, "ready", "interrupt")
	sendUntilMsgIsReceived(t, cmd, os.Interrupt, "ready", "interrupt", "interrupt")

	// We can't check the last interrupt message since the child process
	// may be killed by Terramate with SIGKILL before it gets the signal
	// or it is able to send messages to stdout.
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	errs := make(chan error)
	go func() {
		errs <- cmd.wait()
		close(errs)
	}()

	for ctx.Err() == nil {
		t.Log("sending last interrupt signal to terramate")
		cmd.signalGroup(os.Interrupt)

		sendctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		select {
		case err := <-errs:
			t.Logf("terramate err: %v", err)
			t.Logf("terramate stdout:\n%s\n", cmd.stdout.String())
			t.Logf("terramate stderr:\n%s\n", cmd.stderr.String())
			assert.Error(t, err)
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
func pollBufferForMsgs(buf *buffer, wantMsgs ...string) error {
	const (
		timeout      = 10 * time.Second
		pollInterval = 30 * time.Millisecond
	)

	var elapsed time.Duration

	for {
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

func sendUntilMsgIsReceived(t *testing.T, cmd *testCmd, signal os.Signal, msgs ...string) {
	// For some reason in some environments, specially CI ones,
	// signals are not being delivered, so we retry sending the signal.
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	for {
		cmd.signalGroup(signal)
		err := pollBufferForMsgs(cmd.stdout, msgs...)
		if err == nil {
			return
		}
		if ctx.Err() != nil {
			t.Fatal(err)
		}
	}
}
