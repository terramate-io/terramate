// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"testing"

	. "github.com/terramate-io/terramate/e2etests/internal/runner"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestRunTerragruntIncludeAllDependents(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		"f:terragrunt.hcl:" + Doc(
			Block("terraform"),
		).String(),
		"f:stack-a/terragrunt.hcl:" + Doc(
			Block("terraform",
				Str("source", "github.com/example/module"),
			),
			Block("include",
				Labels("root"),
				Expr("path", `find_in_parent_folders()`),
			),
		).String(),
		"f:stack-b/terragrunt.hcl:" + Doc(
			Block("terraform",
				Str("source", "github.com/example/module"),
			),
			Block("include",
				Labels("root"),
				Expr("path", `find_in_parent_folders()`),
			),
			Block("dependencies",
				Expr("paths", `["../stack-a"]`),
			),
		).String(),
		"f:stack-c/terragrunt.hcl:" + Doc(
			Block("terraform",
				Str("source", "github.com/example/module"),
			),
			Block("include",
				Labels("root"),
				Expr("path", `find_in_parent_folders()`),
			),
			Block("dependencies",
				Expr("paths", `["../stack-b"]`),
			),
		).String(),
	})

	// Create Terramate stacks from Terragrunt modules
	cli := NewCLI(t, s.RootDir())
	AssertRunResult(t, cli.Run("create", "--all-terragrunt"), RunExpected{
		IgnoreStdout: true,
	})

	s.Git().CommitAll("init stacks")

	// Change stack-a to select it
	s.RootEntry().CreateFile("stack-a/test.txt", "change")
	s.Git().Add("stack-a/test.txt")
	s.Git().Commit("change stack-a")

	// Run on changed stacks with all dependents
	res := cli.Run("run", "--changed", "--include-all-dependents", "--", "echo", "executed")
	AssertRunResult(t, res, RunExpected{
		Stdout: "executed\nexecuted\nexecuted\n",
		StderrRegexes: []string{
			`Entering stack in /stack-a`,
			`Entering stack in /stack-b`,
			`Entering stack in /stack-c`,
		},
	})
}

func TestRunTerragruntOnlyAllDependents(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		"f:terragrunt.hcl:" + Doc(
			Block("terraform"),
		).String(),
		"f:stack-a/terragrunt.hcl:" + Doc(
			Block("terraform",
				Str("source", "github.com/example/module"),
			),
		).String(),
		"f:stack-b/terragrunt.hcl:" + Doc(
			Block("terraform",
				Str("source", "github.com/example/module"),
			),
			Block("dependencies",
				Expr("paths", `["../stack-a"]`),
			),
		).String(),
	})

	cli := NewCLI(t, s.RootDir())
	AssertRunResult(t, cli.Run("create", "--all-terragrunt"), RunExpected{
		IgnoreStdout: true,
	})
	s.Git().CommitAll("init stacks")

	// Change stack-a
	s.RootEntry().CreateFile("stack-a/test.txt", "change")
	s.Git().Add("stack-a/test.txt")
	s.Git().Commit("change stack-a")

	// Run only on dependents of changed stacks (not including the changed stack itself)
	res := cli.Run("run", "--changed", "--only-all-dependents", "--", "echo", "executed")
	AssertRunResult(t, res, RunExpected{
		Stdout: "executed\n",
		StderrRegexes: []string{
			`Entering stack in /stack-b`,
		},
	})
}

func TestRunTerragruntIncludeDependencies(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		"f:terragrunt.hcl:" + Doc(
			Block("terraform"),
		).String(),
		"f:stack-a/terragrunt.hcl:" + Doc(
			Block("terraform",
				Str("source", "github.com/example/module"),
			),
		).String(),
		"f:stack-b/terragrunt.hcl:" + Doc(
			Block("terraform",
				Str("source", "github.com/example/module"),
			),
			Block("dependencies",
				Expr("paths", `["../stack-a"]`),
			),
		).String(),
		"f:stack-c/terragrunt.hcl:" + Doc(
			Block("terraform",
				Str("source", "github.com/example/module"),
			),
			Block("dependencies",
				Expr("paths", `["../stack-b"]`),
			),
		).String(),
	})

	cli := NewCLI(t, s.RootDir())
	AssertRunResult(t, cli.Run("create", "--all-terragrunt"), RunExpected{
		IgnoreStdout: true,
	})
	s.Git().CommitAll("init stacks")

	// Change stack-c
	s.RootEntry().CreateFile("stack-c/test.txt", "change")
	s.Git().Add("stack-c/test.txt")
	s.Git().Commit("change stack-c")

	// Run on changed stacks with dependencies
	res := cli.Run("run", "--changed", "--include-all-dependencies", "--", "echo", "executed")
	AssertRunResult(t, res, RunExpected{
		Stdout: "executed\nexecuted\nexecuted\n",
		StderrRegexes: []string{
			`Entering stack in /stack-a`,
			`Entering stack in /stack-b`,
			`Entering stack in /stack-c`,
		},
	})
}

func TestRunTerragruntOnlyDependencies(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		"f:terragrunt.hcl:" + Doc(
			Block("terraform"),
		).String(),
		"f:stack-a/terragrunt.hcl:" + Doc(
			Block("terraform",
				Str("source", "github.com/example/module"),
			),
		).String(),
		"f:stack-b/terragrunt.hcl:" + Doc(
			Block("terraform",
				Str("source", "github.com/example/module"),
			),
			Block("dependencies",
				Expr("paths", `["../stack-a"]`),
			),
		).String(),
	})

	cli := NewCLI(t, s.RootDir())
	AssertRunResult(t, cli.Run("create", "--all-terragrunt"), RunExpected{
		IgnoreStdout: true,
	})
	s.Git().CommitAll("init stacks")

	// Change stack-b
	s.RootEntry().CreateFile("stack-b/test.txt", "change")
	s.Git().Add("stack-b/test.txt")
	s.Git().Commit("change stack-b")

	// Run only on dependencies (not the changed stack itself)
	res := cli.Run("run", "--changed", "--only-all-dependencies", "--", "echo", "executed")
	AssertRunResult(t, res, RunExpected{
		Stdout: "executed\n",
		StderrRegexes: []string{
			`Entering stack in /stack-a`,
		},
	})
}
