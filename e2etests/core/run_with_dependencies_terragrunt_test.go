// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"testing"

	. "github.com/terramate-io/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/hcl"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestRunTerragruntIncludeAllDependents(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		`f:terramate.tm:` + Terramate(
			Config(
				Experiments(hcl.SharingIsCaringExperimentName),
			),
		).String(),
		`f:sharing.tm:` + Block("sharing_backend",
			Labels("default"),
			Expr("type", "terraform"),
			Command("terraform", "output", "-json"),
			Str("filename", "_sharing.tf"),
		).String(),
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

	// Generate code
	s.Generate()

	s.Git().CommitAll("init stacks")
	s.Git().Push("main")
	s.Git().CheckoutNew("test-branch")

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
		`f:terramate.tm:` + Terramate(
			Config(
				Experiments(hcl.SharingIsCaringExperimentName),
			),
		).String(),
		`f:sharing.tm:` + Block("sharing_backend",
			Labels("default"),
			Expr("type", "terraform"),
			Command("terraform", "output", "-json"),
			Str("filename", "_sharing.tf"),
		).String(),
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

	// Generate code
	s.Generate()

	s.Git().CommitAll("init stacks")
	s.Git().Push("main")
	s.Git().CheckoutNew("test-branch")

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
		`f:terramate.tm:` + Terramate(
			Config(
				Experiments(hcl.SharingIsCaringExperimentName),
			),
		).String(),
		`f:sharing.tm:` + Block("sharing_backend",
			Labels("default"),
			Expr("type", "terraform"),
			Command("terraform", "output", "-json"),
			Str("filename", "_sharing.tf"),
		).String(),
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

	// Run on changed stacks with dependencies
	// After our fix, dependencies.paths should NOT widen scope, so we should only run stack-c
	res := cli.Run("run", "--changed", "--include-all-dependencies", "--", "echo", "executed")
	AssertRunResult(t, res, RunExpected{
		Stdout: "executed\n",
		StderrRegexes: []string{
			`Entering stack in /stack-c`,
		},
	})
}

func TestRunTerragruntOnlyDependencies(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		`f:terramate.tm:` + Terramate(
			Config(
				Experiments(hcl.SharingIsCaringExperimentName),
			),
		).String(),
		`f:sharing.tm:` + Block("sharing_backend",
			Labels("default"),
			Expr("type", "terraform"),
			Command("terraform", "output", "-json"),
			Str("filename", "_sharing.tf"),
		).String(),
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
	s.Git().Push("main")
	s.Git().CheckoutNew("test-branch")

	// Change stack-b
	s.RootEntry().CreateFile("stack-b/test.txt", "change")
	s.Git().Add("stack-b/test.txt")
	s.Git().Commit("change stack-b")

	// Run only on dependencies (not the changed stack itself)
	// After our fix, dependencies.paths should NOT widen scope, so we should get no stacks
	res := cli.Run("run", "--changed", "--only-all-dependencies", "--", "echo", "executed")
	AssertRunResult(t, res, RunExpected{
		Stdout:        "",
		StderrRegexes: []string{},
	})
}

func TestRunTerragruntDependencyFlagsWithoutExperiments(t *testing.T) {
	t.Parallel()

	// This test verifies that dependency flags work correctly WITHOUT any
	// experiments enabled. Our new implementation reads directly from
	// mod.DependencyBlocks, so it doesn't require the outputs-sharing experiment.
	s := sandbox.New(t)
	s.BuildTree([]string{
		// Note: No experiments enabled - dependency tracking should still work
		`f:terramate.tm:` + Terramate(
			Config(),
		).String(),
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

	cli := NewCLI(t, s.RootDir())
	AssertRunResult(t, cli.Run("create", "--all-terragrunt"), RunExpected{
		IgnoreStdout: true,
	})

	// No Generate() needed - no generate blocks, input/output blocks, or sharing backends.
	// Dependency tracking works directly from mod.DependencyBlocks without generating files.

	s.Git().CommitAll("init stacks")
	s.Git().Push("main")
	s.Git().CheckoutNew("test-branch")

	// Change stack-a
	s.RootEntry().CreateFile("stack-a/test.txt", "change")
	s.Git().Add("stack-a/test.txt")
	s.Git().Commit("change stack-a")

	// Test dependency flags without any experiments - should work via mod.DependencyBlocks
	res := cli.Run("run", "--changed", "--include-all-dependents", "--terragrunt", "--", "echo", "executed")
	AssertRunResult(t, res, RunExpected{
		Stdout: "executed\nexecuted\nexecuted\n",
		StderrRegexes: []string{
			`Entering stack in /stack-a`,
			`Entering stack in /stack-b`,
			`Entering stack in /stack-c`,
		},
	})
}

func TestRunTerragruntDependencyFlagsWithScriptsExperiment(t *testing.T) {
	t.Parallel()

	// This test verifies that dependency flags work correctly with the "scripts"
	// experiment enabled (but not outputs-sharing). This ensures experiments don't
	// interfere with Terragrunt dependency tracking.
	s := sandbox.New(t)
	s.BuildTree([]string{
		`f:terramate.tm:` + Terramate(
			Config(
				Experiments("scripts"),
			),
		).String(),
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

	// No Generate() needed - no generate blocks, input/output blocks, or sharing backends.
	// Dependency tracking works directly from mod.DependencyBlocks without generating files.

	s.Git().CommitAll("init stacks")
	s.Git().Push("main")
	s.Git().CheckoutNew("test-branch")

	// Change stack-a
	s.RootEntry().CreateFile("stack-a/test.txt", "change")
	s.Git().Add("stack-a/test.txt")
	s.Git().Commit("change stack-a")

	// Test dependency flags with scripts experiment - should work
	res := cli.Run("run", "--changed", "--include-all-dependents", "--terragrunt", "--", "echo", "executed")
	AssertRunResult(t, res, RunExpected{
		Stdout: "executed\nexecuted\n",
		StderrRegexes: []string{
			`Entering stack in /stack-a`,
			`Entering stack in /stack-b`,
		},
	})
}

func TestRunTerragruntDependencyFlagsWithSharingBackend(t *testing.T) {
	t.Parallel()

	// This test verifies that dependency flags still work correctly when
	// sharing_backend blocks ARE defined (the existing happy path).
	// This ensures we didn't break anything with our fix.
	s := sandbox.New(t)
	s.BuildTree([]string{
		`f:terramate.tm:` + Terramate(
			Config(
				Experiments(hcl.SharingIsCaringExperimentName),
			),
		).String(),
		`f:sharing.tm:` + Block("sharing_backend",
			Labels("default"),
			Expr("type", "terraform"),
			Command("terraform", "output", "-json"),
			Str("filename", "_sharing.tf"),
		).String(),
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

	cli := NewCLI(t, s.RootDir())
	AssertRunResult(t, cli.Run("create", "--all-terragrunt"), RunExpected{
		IgnoreStdout: true,
	})

	// Generate code with sharing backend - should work
	s.Generate()

	s.Git().CommitAll("init stacks")
	s.Git().Push("main")
	s.Git().CheckoutNew("test-branch")

	// Change stack-a
	s.RootEntry().CreateFile("stack-a/test.txt", "change")
	s.Git().Add("stack-a/test.txt")
	s.Git().Commit("change stack-a")

	// Test --include-all-dependents with sharing backend (should work same as without)
	res := cli.Run("run", "--changed", "--include-all-dependents", "--terragrunt", "--", "echo", "executed")
	AssertRunResult(t, res, RunExpected{
		Stdout: "executed\nexecuted\nexecuted\n",
		StderrRegexes: []string{
			`Entering stack in /stack-a`,
			`Entering stack in /stack-b`,
			`Entering stack in /stack-c`,
		},
	})
}
