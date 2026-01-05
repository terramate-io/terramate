// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"testing"

	. "github.com/terramate-io/terramate/e2etests/internal/runner"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestListTerragruntIncludeAllDependents(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		"f:terragrunt.hcl:" + Doc(
			Block("terraform",
				Str("source", "github.com/example/module"),
			),
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
			Block("dependency",
				Labels("stack_a"),
				Str("config_path", "../stack-a"),
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
			Block("dependency",
				Labels("stack_b"),
				Str("config_path", "../stack-b"),
			),
		).String(),
	})

	// Create Terramate stacks from Terragrunt modules
	cli := NewCLI(t, s.RootDir())
	AssertRunResult(t, cli.Run("create", "--all-terragrunt"), RunExpected{
		IgnoreStdout: true,
	})

	s.Git().CommitAll("init stacks")
	s.Git().Push("main")
	s.Git().CheckoutNew("test-branch")

	// Change stack-a to select it
	s.RootEntry().CreateFile("stack-a/test.txt", "change")
	s.Git().Add("stack-a/test.txt")
	s.Git().Commit("change stack-a")

	// List changed stacks with all dependents
	res := cli.Run("list", "--changed", "--include-all-dependents")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-a", "stack-b", "stack-c"),
	})
}

func TestListTerragruntOnlyAllDependents(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		"f:terragrunt.hcl:" + Doc(
			Block("terraform",
				Str("source", "github.com/example/module"),
			),
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
			Block("dependency",
				Labels("stack_a"),
				Str("config_path", "../stack-a"),
			),
		).String(),
	})

	cli := NewCLI(t, s.RootDir())
	AssertRunResult(t, cli.Run("create", "--all-terragrunt"), RunExpected{
		IgnoreStdout: true,
	})
	s.Git().CommitAll("init stacks")
	s.Git().Push("main")
	s.Git().CheckoutNew("test-branch")

	// Change stack-a
	s.RootEntry().CreateFile("stack-a/test.txt", "change")
	s.Git().Add("stack-a/test.txt")
	s.Git().Commit("change stack-a")

	// List only dependents of changed stacks (not including the changed stack itself)
	res := cli.Run("list", "--changed", "--only-all-dependents")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-b"),
	})
}

func TestListTerragruntIncludeDependencies(t *testing.T) {
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
	s.Git().Push("main")
	s.Git().CheckoutNew("test-branch")

	// Change stack-c
	s.RootEntry().CreateFile("stack-c/test.txt", "change")
	s.Git().Add("stack-c/test.txt")
	s.Git().Commit("change stack-c")

	// List changed stacks with dependencies
	// After our fix, dependencies.paths should NOT widen scope, so we should only get stack-c
	res := cli.Run("list", "--changed", "--include-all-dependencies")
	AssertRunResult(t, res, RunExpected{
		Stdout:      nljoin("stack-c"),
		StderrRegex: "incomplete root module",
	})
}

func TestListTerragruntOnlyDependencies(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		"f:terragrunt.hcl:" + Doc(
			Block("terraform",
				Str("source", "github.com/example/module"),
			),
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
	s.Git().Push("main")
	s.Git().CheckoutNew("test-branch")

	// Change stack-b
	s.RootEntry().CreateFile("stack-b/test.txt", "change")
	s.Git().Add("stack-b/test.txt")
	s.Git().Commit("change stack-b")

	// List only dependencies (not the changed stack itself)
	// After our fix, dependencies.paths should NOT widen scope, so we should get no stacks
	res := cli.Run("list", "--changed", "--only-all-dependencies")
	AssertRunResult(t, res, RunExpected{
		Stdout: "",
	})
}

func TestListTerragruntDependenciesPathsDoNotWidenScope(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		"f:terragrunt.hcl:" + Doc(
			Block("terraform",
				Str("source", "github.com/example/module"),
			),
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
	s.Git().Push("main")
	s.Git().CheckoutNew("test-branch")

	// Change stack-c
	s.RootEntry().CreateFile("stack-c/test.txt", "change")
	s.Git().Add("stack-c/test.txt")
	s.Git().Commit("change stack-c")

	// NEGATIVE TEST: dependencies.paths should NOT widen scope
	// With --include-all-dependencies, we should only get stack-c itself
	// because dependencies.paths is for ordering only, not data dependencies
	res := cli.Run("list", "--changed", "--include-all-dependencies")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-c"),
	})
}
