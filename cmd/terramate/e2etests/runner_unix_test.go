// Copyright 2022 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// build flags for unix below were taken from:
// https://github.com/golang/go/blob/master/src/cmd/dist/build.go#L943
// We can replace them by "unix" if we support only go >= 1.19 later.
//go:build aix || android || darwin || dragonfly || freebsd || hurd || illumos || ios || linux || netbsd || openbsd || solaris

package e2etest

import (
	"os"
	"syscall"

	"github.com/madlambda/spells/assert"
)

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
