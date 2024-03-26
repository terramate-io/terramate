// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build !darwin

package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/madlambda/spells/assert"
)

const hangTimeout = time.Minute

// RunMode is a run mode for the command.
type RunMode int

const (
	// NormalRun executes any command and wait for its completion.
	NormalRun RunMode = iota
	// HangRun executes an specialized command which never terminates and ignore all signals.
	HangRun
	// SleepRun executes an specialized command which sleeps for a configured amount of seconds.
	SleepRun
)

// RunFixture defines a runner fixture for specialized modes of run.
type RunFixture struct {
	rootdir string
	flags   []string
	mode    RunMode

	cli     *CLI
	Command []string
}

func (tm CLI) helperPath() string {
	return filepath.Join(toolsetTestPath, "helper") + platExeSuffix()
}

// NewRunFixture returns a new runFixture ready to be executed which supports 3 execution modes:
//   - [NormalMode]
//   - [HangRun]
//   - [SleepRun]
//
// In the case of [HangRun] mode, the steps below will happen:
//  1. The helper `hang` command is invoked.
//  2. The test will poll the stdout buffer waiting for a "ready" message.
//  3. When the process is ready, the testing will send CTRL-C twice and wait for the process acknowledge.
//  4. The test will wait for [hangTimeout] seconds for a graceful exit, otherwise the process is killed with SIGKILL.
//
// In the case of [SleepRun] mode, the steps below will happen:
//  1. The helper `sleep` command is invoked.
//  2. The test will poll the stdout buffer waiting for a "ready" message.
//  3. When the process is ready, the testing will send a single CTRL-C.
//  4. The test will wait for process graceful exit.
//
// Usage:
//
//	cli := NewCli(t, chdir)
//	fixture := NewRunFixture(hangRun, s.RootDir(), "--cloud-sync-deployment")
//	result = fixture.Run()
//	AssertRunResult(t, result, expected)
func (tm *CLI) NewRunFixture(mode RunMode, rootdir string, flags ...string) RunFixture {
	return RunFixture{
		rootdir: rootdir,
		cli:     tm,
		flags:   flags,
		mode:    mode,
	}
}

// Run the fixture.
func (run *RunFixture) Run() RunResult {
	t := run.cli.t
	t.Helper()

	args := []string{"run"}
	args = append(args, run.flags...)
	args = append(args, "--")

	switch run.mode {
	case NormalRun:
		cmd := run.Command
		if len(cmd) == 0 {
			// in normalMode the user can override the invoked command.
			cmd = append(cmd, run.cli.helperPath(), "stack-abs-path", run.rootdir)
		}
		args = append(args, cmd...)
	case HangRun:
		args = append(args, run.cli.helperPath(), "hang")
	case SleepRun:
		args = append(args, run.cli.helperPath(), "sleep", "1m")
	default:
		t.Fatalf("unexpected run mode: %d", run.mode)
	}

	exec := run.cli.NewCmd(args...)

	switch run.mode {
	case NormalRun:
		exec.Start()
		_ = exec.Wait()
	case HangRun:
		testHangProcess(t, exec)
	case SleepRun:
		testSleepProcess(t, exec)
	}

	return RunResult{
		Cmd:    strings.Join(args, " "),
		Stdout: exec.Stdout.String(),
		Stderr: exec.Stderr.String(),
		Status: exec.ExitCode(),
	}
}

func testHangProcess(t *testing.T, exec *Cmd) {
	exec.Setpgid()
	exec.Start()
	errs := make(chan error)
	go func() {
		errs <- exec.Wait()
		close(errs)
	}()
	assert.NoError(t, PollBufferForMsgs(exec.Stdout, errs, "ready"))
	SendUntilMsgIsReceived(t, exec, os.Interrupt, "ready", "interrupt")
	SendUntilMsgIsReceived(t, exec, os.Interrupt, "ready", "interrupt", "interrupt")

	// We can't check the last interrupt message since the child process
	// may be killed by Terramate with SIGKILL before it gets the signal
	// or it is able to send messages to stdout.
	ctx, cancel := context.WithTimeout(context.Background(), hangTimeout)
	defer cancel()

outer:
	for ctx.Err() == nil {
		t.Log("sending last interrupt signal to terramate")
		exec.SignalGroup(os.Interrupt)

		sendctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		select {
		case err := <-errs:
			t.Logf("terramate err: %v", err)
			t.Logf("terramate stdout:\n%s\n", exec.Stdout.String())
			t.Logf("terramate stderr:\n%s\n", exec.Stderr.String())
			assert.Error(t, err)
			break outer
		case <-sendctx.Done():
			t.Log("terramate still running, re-sending interrupt")
		}
	}

	t.Logf("terramate stdout:\n%s\n", exec.Stdout.String())
	t.Logf("terramate stderr:\n%s\n", exec.Stderr.String())
}

func testSleepProcess(t *testing.T, exec *Cmd) {
	exec.Setpgid()
	exec.Start()
	done := make(chan error)
	go func() {
		done <- exec.Wait()
	}()
	assert.NoError(t, PollBufferForMsgs(exec.Stdout, done, "ready"))
	exec.SignalGroup(os.Interrupt)
	<-done
}

// PollBufferForMsgs will check if each message is present on the buffer
// on signal handling sometimes we get extraneous signals on the test process
// like "urgent I/O condition". This function will ignore any unknown messages
// in between but check that at least all msgs where received in the provided
// order (but ignoring unknown messages in between).
func PollBufferForMsgs(buf *buffer, done chan error, wantMsgs ...string) error {
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

// SendUntilMsgIsReceived send a signal until the provided messages are read in the output.
func SendUntilMsgIsReceived(t *testing.T, cmd *Cmd, signal os.Signal, msgs ...string) {
	t.Helper()
	// For some reason in some environments, specially CI ones,
	// signals are not being delivered, so we retry sending the signal.
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	for {
		cmd.SignalGroup(signal)
		err := PollBufferForMsgs(cmd.Stdout, make(chan error), msgs...)
		if err == nil {
			return
		}
		if ctx.Err() != nil {
			t.Fatal(err)
		}
	}
}
