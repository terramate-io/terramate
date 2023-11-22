// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package e2etest

import (
	"testing"

	"github.com/terramate-io/terramate/test/sandbox"
)

func TestLoggingChangeChannel(t *testing.T) {
	t.Parallel()
	s := sandbox.NoGit(t, true)
	cli := newCLI(t, s.RootDir())
	cli.loglevel = "debug"

	assertRunResult(t, cli.listStacks(), runExpected{
		StderrRegex: "DBG",
	})
	assertRunResult(t, cli.listStacks("--log-destination", "stderr"), runExpected{
		StderrRegex: "DBG",
	})
	assertRunResult(t, cli.listStacks("--log-destination", "stdout"), runExpected{
		StdoutRegex: "DBG",
	})
}
