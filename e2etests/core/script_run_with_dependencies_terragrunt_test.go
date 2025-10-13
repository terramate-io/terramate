// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"testing"

	. "github.com/terramate-io/terramate/e2etests/internal/runner"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestScriptRunTerragruntIncludeAllDependents(t *testing.T) {
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

	// Enable scripts experiment and add a script to all stacks
	s.RootEntry().CreateFile("terramate.tm.hcl", Terramate(
		Config(
			Experiments("scripts"),
		),
	).String())

	s.RootEntry().CreateFile("scripts.tm", Block("script",
		Labels("test"),
		Expr("description", `"Test script"`),
		Block("job",
			Expr("command", `["echo", "executed from ${terramate.stack.path.absolute}"]`),
		),
	).String())

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

	// Run script on changed stacks with all dependents
	res := cli.Run("script", "run", "--changed", "--include-all-dependents", "test")
	AssertRunResult(t, res, RunExpected{
		StdoutRegexes: []string{
			`executed from.*stack-a`,
			`executed from.*stack-b`,
			`executed from.*stack-c`,
		},
		IgnoreStderr: true,
	})
}

func TestScriptRunTerragruntOnlyAllDependents(t *testing.T) {
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

	// Enable scripts experiment and add a script to all stacks
	s.RootEntry().CreateFile("terramate.tm.hcl", Terramate(
		Config(
			Experiments("scripts"),
		),
	).String())

	s.RootEntry().CreateFile("scripts.tm", Block("script",
		Labels("test"),
		Expr("description", `"Test script"`),
		Block("job",
			Expr("command", `["echo", "executed from ${terramate.stack.path.absolute}"]`),
		),
	).String())

	cli := NewCLI(t, s.RootDir())
	AssertRunResult(t, cli.Run("create", "--all-terragrunt"), RunExpected{
		IgnoreStdout: true,
	})
	s.Git().CommitAll("init stacks")

	// Change stack-a
	s.RootEntry().CreateFile("stack-a/test.txt", "change")
	s.Git().Add("stack-a/test.txt")
	s.Git().Commit("change stack-a")

	// Run script only on dependents of changed stacks (not including the changed stack itself)
	res := cli.Run("script", "run", "--changed", "--only-all-dependents", "test")
	AssertRunResult(t, res, RunExpected{
		StdoutRegexes: []string{
			`executed from.*stack-b`,
		},
		IgnoreStderr: true,
	})
}

func TestScriptRunTerragruntIncludeDependencies(t *testing.T) {
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

	// Enable scripts experiment and add a script to all stacks
	s.RootEntry().CreateFile("terramate.tm.hcl", Terramate(
		Config(
			Experiments("scripts"),
		),
	).String())

	s.RootEntry().CreateFile("scripts.tm", Block("script",
		Labels("test"),
		Expr("description", `"Test script"`),
		Block("job",
			Expr("command", `["echo", "executed from ${terramate.stack.path.absolute}"]`),
		),
	).String())

	cli := NewCLI(t, s.RootDir())
	AssertRunResult(t, cli.Run("create", "--all-terragrunt"), RunExpected{
		IgnoreStdout: true,
	})
	s.Git().CommitAll("init stacks")

	// Change stack-c
	s.RootEntry().CreateFile("stack-c/test.txt", "change")
	s.Git().Add("stack-c/test.txt")
	s.Git().Commit("change stack-c")

	// Run script on changed stacks with dependencies
	res := cli.Run("script", "run", "--changed", "--include-all-dependencies", "test")
	AssertRunResult(t, res, RunExpected{
		StdoutRegexes: []string{
			`executed from.*stack-a`,
			`executed from.*stack-b`,
			`executed from.*stack-c`,
		},
		IgnoreStderr: true,
	})
}

func TestScriptRunTerragruntOnlyDependencies(t *testing.T) {
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

	// Enable scripts experiment and add a script to all stacks
	s.RootEntry().CreateFile("terramate.tm.hcl", Terramate(
		Config(
			Experiments("scripts"),
		),
	).String())

	s.RootEntry().CreateFile("scripts.tm", Block("script",
		Labels("test"),
		Expr("description", `"Test script"`),
		Block("job",
			Expr("command", `["echo", "executed from ${terramate.stack.path.absolute}"]`),
		),
	).String())

	cli := NewCLI(t, s.RootDir())
	AssertRunResult(t, cli.Run("create", "--all-terragrunt"), RunExpected{
		IgnoreStdout: true,
	})
	s.Git().CommitAll("init stacks")

	// Change stack-b
	s.RootEntry().CreateFile("stack-b/test.txt", "change")
	s.Git().Add("stack-b/test.txt")
	s.Git().Commit("change stack-b")

	// Run script only on dependencies (not the changed stack itself)
	res := cli.Run("script", "run", "--changed", "--only-all-dependencies", "test")
	AssertRunResult(t, res, RunExpected{
		StdoutRegexes: []string{
			`executed from.*stack-a`,
		},
		IgnoreStderr: true,
	})
}
