//go:build windows

package e2etest

import (
	"os"
	"syscall"

	"github.com/madlambda/spells/assert"
)

var (
	libkernel32                  = syscall.MustLoadDLL("kernel32")
	procGenerateConsoleCtrlEvent = libkernel32.MustFindProc("GenerateConsoleCtrlEvent")
)

func (tc *testCmd) setpgid() {
	tc.cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

func (tc *testCmd) signalGroup(s os.Signal) {
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
