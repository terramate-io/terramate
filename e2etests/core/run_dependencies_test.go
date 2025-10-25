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

func TestRunIncludeAllDependents(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		`s:stack-a:tags=["base"]`,
		`s:stack-b:after=["/stack-a"]`,
		`s:stack-c:after=["/stack-b"]`,
		`s:stack-d:after=["/stack-c"]`,
	})
	s.Git().CommitAll("initial commit")
	s.Git().Push("main")
	s.Git().CheckoutNew("test-branch")

	// Change stack-a to select it
	s.RootEntry().CreateFile("stack-a/test.txt", "change")
	s.Git().Add("stack-a/test.txt")
	s.Git().Commit("change stack-a")

	cli := NewCLI(t, s.RootDir())

	// Select only stack-a via changed, include all dependents
	res := cli.Run("list", "--changed", "--include-all-dependents")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-a", "stack-b", "stack-c", "stack-d"),
	})

	// Test with tags: select stack-a via tag, include all dependents
	res = cli.Run("list", "--tags", "base", "--include-all-dependents")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-a", "stack-b", "stack-c", "stack-d"),
	})
}

func TestRunOnlyAllDependents(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		`s:stack-a:tags=["base"]`,
		`s:stack-b:after=["/stack-a"]`,
		`s:stack-c:after=["/stack-b"]`,
		`s:stack-d`,
	})

	cli := NewCLI(t, s.RootDir())

	// Select stack-a via tag, replace with only dependents (not including stack-a itself)
	res := cli.Run("list", "--tags", "base", "--only-all-dependents")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-b", "stack-c"),
	})
}

func TestRunIncludeDirectDependents(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		`s:stack-a:tags=["base"]`,
		`s:stack-b:after=["/stack-a"]`,
		`s:stack-c:after=["/stack-a"]`,
		`s:stack-d:after=["/stack-b"]`,
	})

	cli := NewCLI(t, s.RootDir())

	// Select stack-a via tag, include only direct dependents (b and c, but not d)
	res := cli.Run("list", "--tags", "base", "--include-direct-dependents")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-a", "stack-b", "stack-c"),
	})
}

func TestRunOnlyDirectDependents(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		`s:stack-a:tags=["base"]`,
		`s:stack-b:after=["/stack-a"]`,
		`s:stack-c:after=["/stack-a"]`,
		`s:stack-d:after=["/stack-b"]`,
	})

	cli := NewCLI(t, s.RootDir())

	// Replace selection with only direct dependents
	res := cli.Run("list", "--tags", "base", "--only-direct-dependents")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-b", "stack-c"),
	})
}

func TestRunIncludeDependencies(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		"s:stack-a",
		`s:stack-b:after=["/stack-a"]`,
		`s:stack-c:after=["/stack-b"];tags=["leaf"]`,
	})

	cli := NewCLI(t, s.RootDir())

	// Select stack-c via tag, include all its dependencies
	res := cli.Run("list", "--tags", "leaf", "--include-all-dependencies")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-a", "stack-b", "stack-c"),
	})
}

func TestRunOnlyDependencies(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		"s:stack-a",
		`s:stack-b:after=["/stack-a"]`,
		`s:stack-c:after=["/stack-b"];tags=["leaf"]`,
	})

	cli := NewCLI(t, s.RootDir())

	// Replace selection with only dependencies
	res := cli.Run("list", "--tags", "leaf", "--only-all-dependencies")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-a", "stack-b"),
	})
}

func TestRunExcludeDependencies(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		"s:stack-a",
		`s:stack-b:after=["/stack-a"]`,
		`s:stack-c:after=["/stack-b"]`,
		"s:stack-d",
	})

	cli := NewCLI(t, s.RootDir())

	// Select all stacks, but exclude dependencies of stack-c
	res := cli.Run("list", "--exclude-all-dependencies")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-c", "stack-d"),
	})
}

func TestRunExcludeDependents(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		"s:stack-a",
		`s:stack-b:after=["/stack-a"]`,
		`s:stack-c:after=["/stack-b"]`,
		"s:stack-d",
	})

	cli := NewCLI(t, s.RootDir())

	// Select all stacks, exclude dependents of stack-a
	res := cli.Run("list", "--exclude-all-dependents")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-a", "stack-d"),
	})
}

func TestRunDependenciesWithOutputSharing(t *testing.T) {
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
		`s:stack-a:id=stack-a;tags=["provider"]`,
		`s:stack-b:id=stack-b;tags=["consumer"]`,
		"s:stack-c:id=stack-c",
	})

	// stack-b depends on stack-a via output sharing
	s.RootEntry().CreateFile("stack-a/outputs.tm", Block("output",
		Labels("myoutput"),
		Str("backend", "default"),
		Expr("value", "some.value"),
	).String())

	s.RootEntry().CreateFile("stack-b/inputs.tm", Block("input",
		Labels("myinput"),
		Str("backend", "default"),
		Expr("value", "outputs.myoutput.value"),
		Str("from_stack_id", "stack-a"),
	).String())

	cli := NewCLI(t, s.RootDir())

	// Select stack-b via tag, include all dependents (none in this case)
	res := cli.Run("list", "--tags", "consumer", "--include-all-dependents")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-b"),
	})

	// Select stack-a via tag, include all dependents (should include stack-b)
	res = cli.Run("list", "--tags", "provider", "--include-all-dependents")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-a", "stack-b"),
	})

	// Select stack-b via tag, include dependencies (should include stack-a)
	res = cli.Run("list", "--tags", "consumer", "--include-all-dependencies")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-a", "stack-b"),
	})
}

func TestRunDependenciesWithChangedStacks(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		`s:stack-a:tags=["changed-test"]`,
		`s:stack-b:after=["/stack-a"]`,
		`s:stack-c:after=["/stack-b"]`,
	})
	s.Git().CommitAll("initial commit")
	s.Git().Push("main")
	s.Git().CheckoutNew("test-branch")

	// Make a change to stack-a
	s.RootEntry().CreateFile("stack-a/test.txt", "changed")
	s.Git().Add("stack-a/test.txt")
	s.Git().Commit("change stack-a")

	cli := NewCLI(t, s.RootDir())

	// List changed stacks with all dependents
	res := cli.Run("list", "--changed", "--include-all-dependents")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-a", "stack-b", "stack-c"),
	})

	// List only direct dependents of changed stacks
	res = cli.Run("list", "--changed", "--only-direct-dependents")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-b"),
	})

	// Test with tag selection instead
	res = cli.Run("list", "--tags", "changed-test", "--include-all-dependents")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-a", "stack-b", "stack-c"),
	})
}

func TestRunDependenciesMultipleDependents(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		`s:stack-a:tags=["base"]`,
		`s:stack-b:tags=["base"]`,
		`s:stack-c:after=["/stack-a", "/stack-b"];tags=["leaf"]`,
		`s:stack-d:after=["/stack-c"]`,
	})

	cli := NewCLI(t, s.RootDir())

	// Select stack-a and stack-b via tags, include all dependents
	res := cli.Run("list", "--tags", "base", "--include-all-dependents")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-a", "stack-b", "stack-c", "stack-d"),
	})

	// Select stack-c via tag, include dependencies
	res = cli.Run("list", "--tags", "leaf", "--include-all-dependencies")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-a", "stack-b", "stack-c"),
	})
}

func TestRunDeprecatedOutputDependenciesFlags(t *testing.T) {
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
		"s:stack-a:id=stack-a",
		`s:stack-b:after=["/stack-a"];tags=["leaf"]`,
	})

	// Create output-sharing dependencies
	s.RootEntry().CreateFile("stack-a/outputs.tm", Block("output",
		Labels("output1"),
		Str("backend", "default"),
		Expr("value", "some.value"),
	).String())

	s.RootEntry().CreateFile("stack-b/inputs.tm", Block("input",
		Labels("input1"),
		Str("backend", "default"),
		Expr("value", "outputs.output1.value"),
		Str("from_stack_id", "stack-a"),
	).String())

	s.Generate()
	s.Git().CommitAll("initial commit")

	cli := NewCLI(t, s.RootDir())

	// Test deprecated --include-output-dependencies flag
	// Should include the output-sharing dependency (stack-a) with the selected stack (stack-b)
	res := cli.Run("list", "--tags", "leaf", "--include-output-dependencies")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-b", "stack-a"),
	})

	// Test deprecated --only-output-dependencies flag
	// Should only return the output-sharing dependency (stack-a), not the selected stack
	res = cli.Run("list", "--tags", "leaf", "--only-output-dependencies")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-a"),
	})
}

func TestRunDeprecatedOutputDependenciesWarnings(t *testing.T) {
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
		"s:stack-a:id=stack-a",
		`s:stack-b:after=["/stack-a"];tags=["leaf"]`,
	})

	// Create output-sharing dependencies
	s.RootEntry().CreateFile("stack-a/outputs.tm", Block("output",
		Labels("output1"),
		Str("backend", "default"),
		Expr("value", "some.value"),
	).String())

	s.RootEntry().CreateFile("stack-b/inputs.tm", Block("input",
		Labels("input1"),
		Str("backend", "default"),
		Expr("value", "outputs.output1.value"),
		Str("from_stack_id", "stack-a"),
	).String())

	s.Generate()
	s.Git().CommitAll("initial commit")

	cli := NewCLI(t, s.RootDir())
	cli.LogLevel = "warn"

	// Test that --include-output-dependencies shows correct deprecation warning
	res := cli.Run("list", "--tags", "leaf", "--include-output-dependencies")
	AssertRunResult(t, res, RunExpected{
		Stdout:      nljoin("stack-b", "stack-a"),
		StderrRegex: ".*--include-output-dependencies is deprecated, use --include-direct-dependencies instead.*",
	})

	// Test that --only-output-dependencies shows correct deprecation warning
	res = cli.Run("list", "--tags", "leaf", "--only-output-dependencies")
	AssertRunResult(t, res, RunExpected{
		Stdout:      nljoin("stack-a"),
		StderrRegex: ".*--only-output-dependencies is deprecated, use --only-direct-dependencies instead.*",
	})
}

func TestRunDependentsWithTags(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		`s:stack-a:tags=["app"]`,
		`s:stack-b:after=["/stack-a"];tags=["db"]`,
		`s:stack-c:after=["/stack-b"];tags=["app"]`,
	})

	cli := NewCLI(t, s.RootDir())

	// Select stacks with tag "app", include all dependents
	res := cli.Run("list", "--tags", "app", "--include-all-dependents")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-a", "stack-b", "stack-c"),
	})
}
