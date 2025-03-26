// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"regexp"
	"testing"

	"github.com/terramate-io/terramate"
	. "github.com/terramate-io/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/test"
)

func TestCLIArgs(t *testing.T) {
	cli := NewCLI(t, test.TempDir(t))
	AssertRunResult(t, cli.Run("--help"), RunExpected{
		Status:      0,
		StdoutRegex: regexp.QuoteMeta("Usage: terramate <command>"),
	})

	AssertRunResult(t, cli.Run("list", "--help"), RunExpected{
		Status:      0,
		StdoutRegex: regexp.QuoteMeta("Usage: terramate list"),
	})

	AssertRunResult(t, cli.Run("--version"), RunExpected{
		Status: 0,
		Stdout: terramate.Version() + "\n",
	})

	AssertRunResult(t, cli.Run("--version", "list"), RunExpected{
		Status:      1,
		StderrRegex: regexp.QuoteMeta(`command list cannot be used with flag --version`),
	})
}
