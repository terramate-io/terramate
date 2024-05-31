// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build windows

package runner

import (
	"os"
	"syscall"

	"github.com/madlambda/spells/assert"
)

var (
	libkernel32                  = syscall.MustLoadDLL("kernel32")
	procGenerateConsoleCtrlEvent = libkernel32.MustFindProc("GenerateConsoleCtrlEvent")
)

// Start will start the command.
func (tc *Cmd) Start() {
	t := tc.t
	t.Helper()

	assert.NoError(t, tc.cmd.Start())
}

// Wait for the command completion.
func (tc *Cmd) Wait() error {
	return tc.cmd.Wait()
}

// SignalGroup sends the signal to all processes part of the cmd group.
func (tc *Cmd) Setpgid() {
	tc.cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// SignalGroup sends a signal to all processes part of the group.
func (tc *Cmd) SignalGroup(s os.Signal) {
	t := tc.t
	t.Helper()

	if s == os.Interrupt {
		r, _, err := procGenerateConsoleCtrlEvent.Call(syscall.CTRL_BREAK_EVENT, uintptr(tc.cmd.Process.Pid))
		if r == 0 {
			assert.NoError(t, err, "Error sending CTRL_C_EVENT")
		}

	} else {
		err := tc.cmd.Process.Signal(s)
		assert.NoError(t, err)
	}
}
