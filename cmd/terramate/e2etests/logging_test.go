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
	"testing"

	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestLoggingChangeChannel(t *testing.T) {
	s := sandbox.New(t)
	cli := newCLI(t, s.RootDir())
	cli.loglevel = "trace"

	assertRunResult(t, cli.listStacks(), runExpected{
		StderrRegex: "TRC",
	})
	assertRunResult(t, cli.listStacks("--log-destination", "stderr"), runExpected{
		StderrRegex: "TRC",
	})
	assertRunResult(t, cli.listStacks("--log-destination", "stdout"), runExpected{
		StdoutRegex: "TRC",
	})
}
