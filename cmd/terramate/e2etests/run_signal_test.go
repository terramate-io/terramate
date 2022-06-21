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

package e2etest

import (
	"os"
	"testing"

	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestRunSendsSigkillIfCmdIgnoresInterruptionSignals(t *testing.T) {
	s := sandbox.New(t)
	s.CreateStack("stack-1")

	git := s.Git()
	git.Add(".")
	git.CommitAll("first commit")

	tm := newCLI(t, s.RootDir())
	cmd := tm.newCmd("run", testHelperBin)
	cmd.start()

	cmd.signal(os.Interrupt)
	// Check it is running

	cmd.signal(os.Interrupt)
	// Check it is running

	cmd.signal(os.Interrupt)
	_ = cmd.wait()
	// err := cmd.wait()
	// validate nature of error returned + validate stdout with expected signals
}
