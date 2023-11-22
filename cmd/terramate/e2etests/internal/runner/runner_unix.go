// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build unix && !darwin

package runner

import (
	"os"
	"syscall"

	"github.com/madlambda/spells/assert"
)

// Start the command.
func (tc *Cmd) Start() {
	t := tc.t
	t.Helper()

	assert.NoError(t, tc.cmd.Start())
}

// Wait for the command completion.
func (tc *Cmd) Wait() error {
	return tc.cmd.Wait()
}

// Setpgid sets the pgid process attribute.
// When set, the command will execute in a new process group id.
func (tc *Cmd) Setpgid() {
	tc.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

// SignalGroup sends the signal to all processes part of the cmd group.
func (tc *Cmd) SignalGroup(s os.Signal) {
	t := tc.t
	t.Helper()

	signal := s.(syscall.Signal)
	// Signalling a group is done by sending the signal to -PID.
	err := syscall.Kill(-tc.cmd.Process.Pid, signal)
	assert.NoError(t, err)
}
