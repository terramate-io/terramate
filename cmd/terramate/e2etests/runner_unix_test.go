//go:build aix || android || darwin || dragonfly || freebsd || hurd || illumos || ios || linux || netbsd || openbsd || solaris

package e2etest

import (
	"os"
	"syscall"

	"github.com/madlambda/spells/assert"
)

// build flags for unix below were taken from:
// https://github.com/golang/go/blob/master/src/cmd/dist/build.go#L943
// We can replace them by "unix" if we support only go >= 1.19 later.

func (tc *testCmd) setpgid() {
	tc.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

func (tc *testCmd) signalGroup(s os.Signal) {
	t := tc.t
	t.Helper()

	signal := s.(syscall.Signal)
	// Signalling a group is done by sending the signal to -PID.
	err := syscall.Kill(-tc.cmd.Process.Pid, signal)
	assert.NoError(t, err)
}
