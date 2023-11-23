// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"testing"

	. "github.com/terramate-io/terramate/cmd/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestLoggingChangeChannel(t *testing.T) {
	t.Parallel()
	s := sandbox.NoGit(t, true)
	cli := NewCLI(t, s.RootDir())
	cli.LogLevel = "trace"

	AssertRunResult(t, cli.ListStacks(), RunExpected{
		StderrRegex: "DBG",
	})
	AssertRunResult(t, cli.ListStacks("--log-destination", "stderr"), RunExpected{
		StderrRegex: "DBG",
	})
	AssertRunResult(t, cli.ListStacks("--log-destination", "stdout"), RunExpected{
		StdoutRegex: "DBG",
	})
}
