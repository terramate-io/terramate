// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package e2etest

import (
	"testing"

	"github.com/terramate-io/terramate/test/sandbox"
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
