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
