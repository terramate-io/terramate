// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build !darwin

package e2etest

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/madlambda/spells/assert"
)

const hangTimeout = time.Minute

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
