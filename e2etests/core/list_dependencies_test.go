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

func TestListIncludeAllDependents(t *testing.T) {
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
		`s:stack-a:id=stack-a;tags=["base"]`,
		`s:stack-b:id=stack-b`,
		`s:stack-c:id=stack-c`,
		`s:stack-d:id=stack-d`,
	})

	// Create data dependencies using input.from_stack_id
	s.RootEntry().CreateFile("stack-b/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_a"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-a"),
		),
	).String())
	s.RootEntry().CreateFile("stack-c/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_b"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-b"),
		),
	).String())
	s.RootEntry().CreateFile("stack-d/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_c"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-c"),
		),
	).String())

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

func TestListOnlyAllDependents(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{

		`f:sharing.tm:` + Block("sharing_backend",
			Labels("default"),
			Expr("type", "terraform"),
			Command("terraform", "output", "-json"),
			Str("filename", "_sharing.tf"),
		).String(),
		`f:terramate.tm:` + Terramate(
			Config(
				Experiments(hcl.SharingIsCaringExperimentName),
			),
		).String(),
		`s:stack-a:id=stack-a;tags=["base"]`,
		`s:stack-b:id=stack-b`,
		`s:stack-c:id=stack-c`,
		`s:stack-d`,
	})

	// Create data dependencies using input.from_stack_id
	s.RootEntry().CreateFile("stack-b/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_a"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-a"),
		),
	).String())
	s.RootEntry().CreateFile("stack-c/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_b"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-b"),
		),
	).String())

	cli := NewCLI(t, s.RootDir())

	// Select stack-a via tag, replace with only dependents (not including stack-a itself)
	res := cli.Run("list", "--tags", "base", "--only-all-dependents")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-b", "stack-c"),
	})
}

func TestListIncludeDirectDependents(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{

		`f:sharing.tm:` + Block("sharing_backend",
			Labels("default"),
			Expr("type", "terraform"),
			Command("terraform", "output", "-json"),
			Str("filename", "_sharing.tf"),
		).String(),
		`f:terramate.tm:` + Terramate(
			Config(
				Experiments(hcl.SharingIsCaringExperimentName),
			),
		).String(),
		`s:stack-a:id=stack-a;tags=["base"]`,
		`s:stack-b:id=stack-b`,
		`s:stack-c:id=stack-c`,
		`s:stack-d:id=stack-d`,
	})

	// Create data dependencies: b→a, c→a, d→b
	s.RootEntry().CreateFile("stack-b/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_a"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-a"),
		),
	).String())
	s.RootEntry().CreateFile("stack-c/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_a"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-a"),
		),
	).String())
	s.RootEntry().CreateFile("stack-d/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_b"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-b"),
		),
	).String())

	cli := NewCLI(t, s.RootDir())

	// Select stack-a via tag, include only direct dependents (b and c, but not d)
	res := cli.Run("list", "--tags", "base", "--include-direct-dependents")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-a", "stack-b", "stack-c"),
	})
}

func TestListOnlyDirectDependents(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{

		`f:sharing.tm:` + Block("sharing_backend",
			Labels("default"),
			Expr("type", "terraform"),
			Command("terraform", "output", "-json"),
			Str("filename", "_sharing.tf"),
		).String(),
		`f:terramate.tm:` + Terramate(
			Config(
				Experiments(hcl.SharingIsCaringExperimentName),
			),
		).String(),
		`s:stack-a:id=stack-a;tags=["base"]`,
		`s:stack-b:id=stack-b`,
		`s:stack-c:id=stack-c`,
		`s:stack-d:id=stack-d`,
	})

	// Create data dependencies: b→a, c→a, d→b
	s.RootEntry().CreateFile("stack-b/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_a"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-a"),
		),
	).String())
	s.RootEntry().CreateFile("stack-c/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_a"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-a"),
		),
	).String())
	s.RootEntry().CreateFile("stack-d/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_b"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-b"),
		),
	).String())

	cli := NewCLI(t, s.RootDir())

	// Replace selection with only direct dependents
	res := cli.Run("list", "--tags", "base", "--only-direct-dependents")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-b", "stack-c"),
	})
}

func TestListIncludeDependencies(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{

		`f:sharing.tm:` + Block("sharing_backend",
			Labels("default"),
			Expr("type", "terraform"),
			Command("terraform", "output", "-json"),
			Str("filename", "_sharing.tf"),
		).String(),
		`f:terramate.tm:` + Terramate(
			Config(
				Experiments(hcl.SharingIsCaringExperimentName),
			),
		).String(),
		`s:stack-a:id=stack-a`,
		`s:stack-b:id=stack-b`,
		`s:stack-c:id=stack-c;tags=["leaf"]`,
	})

	// Create data dependencies: c→b→a
	s.RootEntry().CreateFile("stack-b/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_a"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-a"),
		),
	).String())
	s.RootEntry().CreateFile("stack-c/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_b"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-b"),
		),
	).String())

	cli := NewCLI(t, s.RootDir())

	// Select stack-c via tag, include all its dependencies
	res := cli.Run("list", "--tags", "leaf", "--include-all-dependencies")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-a", "stack-b", "stack-c"),
	})
}

func TestListOnlyDependencies(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{

		`f:sharing.tm:` + Block("sharing_backend",
			Labels("default"),
			Expr("type", "terraform"),
			Command("terraform", "output", "-json"),
			Str("filename", "_sharing.tf"),
		).String(),
		`f:terramate.tm:` + Terramate(
			Config(
				Experiments(hcl.SharingIsCaringExperimentName),
			),
		).String(),
		`s:stack-a:id=stack-a`,
		`s:stack-b:id=stack-b`,
		`s:stack-c:id=stack-c;tags=["leaf"]`,
	})

	// Create data dependencies: c→b→a
	s.RootEntry().CreateFile("stack-b/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_a"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-a"),
		),
	).String())
	s.RootEntry().CreateFile("stack-c/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_b"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-b"),
		),
	).String())

	cli := NewCLI(t, s.RootDir())

	// Replace selection with only dependencies
	res := cli.Run("list", "--tags", "leaf", "--only-all-dependencies")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-a", "stack-b"),
	})
}

func TestListExcludeDependencies(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{

		`f:sharing.tm:` + Block("sharing_backend",
			Labels("default"),
			Expr("type", "terraform"),
			Command("terraform", "output", "-json"),
			Str("filename", "_sharing.tf"),
		).String(),
		`f:terramate.tm:` + Terramate(
			Config(
				Experiments(hcl.SharingIsCaringExperimentName),
			),
		).String(),
		`s:stack-a:id=stack-a`,
		`s:stack-b:id=stack-b`,
		`s:stack-c:id=stack-c`,
		"s:stack-d",
	})

	// Create data dependencies: c→b→a
	s.RootEntry().CreateFile("stack-b/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_a"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-a"),
		),
	).String())
	s.RootEntry().CreateFile("stack-c/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_b"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-b"),
		),
	).String())

	cli := NewCLI(t, s.RootDir())

	// Select all stacks, but exclude dependencies of stack-c
	res := cli.Run("list", "--exclude-all-dependencies")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-c", "stack-d"),
	})
}

func TestListExcludeDependents(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{

		`f:sharing.tm:` + Block("sharing_backend",
			Labels("default"),
			Expr("type", "terraform"),
			Command("terraform", "output", "-json"),
			Str("filename", "_sharing.tf"),
		).String(),
		`f:terramate.tm:` + Terramate(
			Config(
				Experiments(hcl.SharingIsCaringExperimentName),
			),
		).String(),
		`s:stack-a:id=stack-a`,
		`s:stack-b:id=stack-b`,
		`s:stack-c:id=stack-c`,
		"s:stack-d",
	})

	// Create data dependencies: c→b→a
	s.RootEntry().CreateFile("stack-b/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_a"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-a"),
		),
	).String())
	s.RootEntry().CreateFile("stack-c/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_b"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-b"),
		),
	).String())

	cli := NewCLI(t, s.RootDir())

	// Select all stacks, exclude dependents of stack-a
	res := cli.Run("list", "--exclude-all-dependents")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-a", "stack-d"),
	})
}

func TestListDependenciesWithOutputSharing(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{

		`f:sharing.tm:` + Block("sharing_backend",
			Labels("default"),
			Expr("type", "terraform"),
			Command("terraform", "output", "-json"),
			Str("filename", "_sharing.tf"),
		).String(),
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

func TestListDependenciesWithChangedStacks(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{

		`f:sharing.tm:` + Block("sharing_backend",
			Labels("default"),
			Expr("type", "terraform"),
			Command("terraform", "output", "-json"),
			Str("filename", "_sharing.tf"),
		).String(),
		`f:terramate.tm:` + Terramate(
			Config(
				Experiments(hcl.SharingIsCaringExperimentName),
			),
		).String(),
		`s:stack-a:id=stack-a;tags=["changed-test"]`,
		`s:stack-b:id=stack-b`,
		`s:stack-c:id=stack-c`,
	})

	// Create data dependencies: c→b→a
	s.RootEntry().CreateFile("stack-b/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_a"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-a"),
		),
	).String())
	s.RootEntry().CreateFile("stack-c/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_b"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-b"),
		),
	).String())

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

func TestListDependenciesMultipleDependents(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{

		`f:sharing.tm:` + Block("sharing_backend",
			Labels("default"),
			Expr("type", "terraform"),
			Command("terraform", "output", "-json"),
			Str("filename", "_sharing.tf"),
		).String(),
		`f:terramate.tm:` + Terramate(
			Config(
				Experiments(hcl.SharingIsCaringExperimentName),
			),
		).String(),
		`s:stack-a:id=stack-a;tags=["base"]`,
		`s:stack-b:id=stack-b;tags=["base"]`,
		`s:stack-c:id=stack-c;tags=["leaf"]`,
		`s:stack-d:id=stack-d`,
	})

	// Create data dependencies: c→a, c→b, d→c
	s.RootEntry().CreateFile("stack-c/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_a"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-a"),
		),
		Block("input",
			Labels("dep_b"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-b"),
		),
	).String())
	s.RootEntry().CreateFile("stack-d/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_c"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-c"),
		),
	).String())

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

func TestListDeprecatedOutputDependenciesFlags(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{

		`f:sharing.tm:` + Block("sharing_backend",
			Labels("default"),
			Expr("type", "terraform"),
			Command("terraform", "output", "-json"),
			Str("filename", "_sharing.tf"),
		).String(),
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

func TestListDeprecatedOutputDependenciesWarnings(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{

		`f:sharing.tm:` + Block("sharing_backend",
			Labels("default"),
			Expr("type", "terraform"),
			Command("terraform", "output", "-json"),
			Str("filename", "_sharing.tf"),
		).String(),
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

func TestListDependentsWithTags(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{

		`f:sharing.tm:` + Block("sharing_backend",
			Labels("default"),
			Expr("type", "terraform"),
			Command("terraform", "output", "-json"),
			Str("filename", "_sharing.tf"),
		).String(),
		`f:terramate.tm:` + Terramate(
			Config(
				Experiments(hcl.SharingIsCaringExperimentName),
			),
		).String(),
		`s:stack-a:id=stack-a;tags=["app"]`,
		`s:stack-b:id=stack-b;tags=["db"]`,
		`s:stack-c:id=stack-c;tags=["app"]`,
	})

	// Create data dependencies: c→b→a
	s.RootEntry().CreateFile("stack-b/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_a"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-a"),
		),
	).String())
	s.RootEntry().CreateFile("stack-c/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_b"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-b"),
		),
	).String())

	cli := NewCLI(t, s.RootDir())

	// Select stacks with tag "app", include all dependents
	res := cli.Run("list", "--tags", "app", "--include-all-dependents")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-a", "stack-b", "stack-c"),
	})
}

func TestListCombinedFiltersOnlyWithExclude(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{

		`f:sharing.tm:` + Block("sharing_backend",
			Labels("default"),
			Expr("type", "terraform"),
			Command("terraform", "output", "-json"),
			Str("filename", "_sharing.tf"),
		).String(),
		`f:terramate.tm:` + Terramate(
			Config(
				Experiments(hcl.SharingIsCaringExperimentName),
			),
		).String(),
		`s:stack-a:id=stack-a`,
		`s:stack-b:id=stack-b`,
		`s:stack-c:id=stack-c`,
		`s:stack-d:id=stack-d;tags=["leaf"]`,
		"s:stack-e",
	})

	// Create data dependencies: d→c→b→a
	s.RootEntry().CreateFile("stack-b/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_a"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-a"),
		),
	).String())
	s.RootEntry().CreateFile("stack-c/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_b"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-b"),
		),
	).String())
	s.RootEntry().CreateFile("stack-d/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_c"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-c"),
		),
	).String())

	cli := NewCLI(t, s.RootDir())

	// Select stack-d via tag, get only its dependencies (a, b, c), then exclude their dependents
	// Dependencies of stack-d: stack-a, stack-b, stack-c
	// After exclude-all-dependents on that set:
	//   - Dependents of stack-a from {a,b,c}: b, c -> exclude b, c
	//   - Dependents of stack-b from {a,b,c}: c -> exclude c
	//   - Dependents of stack-c from {a,b,c}: none
	// Result should be: stack-a (has no dependents in the filtered set)
	res := cli.Run("list", "--tags", "leaf", "--only-all-dependencies", "--exclude-all-dependents")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-a"),
	})
}

func TestListCombinedFiltersIncludeWithExclude(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{

		`f:sharing.tm:` + Block("sharing_backend",
			Labels("default"),
			Expr("type", "terraform"),
			Command("terraform", "output", "-json"),
			Str("filename", "_sharing.tf"),
		).String(),
		`f:terramate.tm:` + Terramate(
			Config(
				Experiments(hcl.SharingIsCaringExperimentName),
			),
		).String(),
		`s:stack-a:id=stack-a;tags=["base"]`,
		`s:stack-b:id=stack-b`,
		`s:stack-c:id=stack-c`,
		`s:stack-d:id=stack-d`,
		"s:stack-e",
	})

	// Create data dependencies: d→c→b→a
	s.RootEntry().CreateFile("stack-b/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_a"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-a"),
		),
	).String())
	s.RootEntry().CreateFile("stack-c/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_b"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-b"),
		),
	).String())
	s.RootEntry().CreateFile("stack-d/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_c"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-c"),
		),
	).String())

	cli := NewCLI(t, s.RootDir())

	// Select stack-a via tag, include all dependents (a, b, c, d), then exclude dependencies
	// Selection after include: stack-a, stack-b, stack-c, stack-d
	// Dependencies of those stacks from the selection:
	//   - stack-a: none
	//   - stack-b: stack-a
	//   - stack-c: stack-a, stack-b
	//   - stack-d: stack-a, stack-b, stack-c
	// After exclude: only stack-d remains (it has dependencies but they get excluded)
	res := cli.Run("list", "--tags", "base", "--include-all-dependents", "--exclude-all-dependencies")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-d"),
	})
}

func TestListCombinedFiltersOnlyDependentsWithExclude(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{

		`f:sharing.tm:` + Block("sharing_backend",
			Labels("default"),
			Expr("type", "terraform"),
			Command("terraform", "output", "-json"),
			Str("filename", "_sharing.tf"),
		).String(),
		`f:terramate.tm:` + Terramate(
			Config(
				Experiments(hcl.SharingIsCaringExperimentName),
			),
		).String(),
		`s:stack-a:id=stack-a;tags=["base"]`,
		`s:stack-b:id=stack-b`,
		`s:stack-c:id=stack-c`,
		`s:stack-d:id=stack-d`,
		"s:stack-e",
	})

	// Create data dependencies: c→b→a, d→b
	s.RootEntry().CreateFile("stack-b/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_a"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-a"),
		),
	).String())
	s.RootEntry().CreateFile("stack-c/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_b"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-b"),
		),
	).String())
	s.RootEntry().CreateFile("stack-d/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_b"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-b"),
		),
	).String())

	cli := NewCLI(t, s.RootDir())

	// Select stack-a via tag, get only its dependents (b, c, d), then exclude dependencies
	// After only-all-dependents: stack-b, stack-c, stack-d
	// Dependencies of those from the filtered set:
	//   - stack-b: none (stack-a not in filtered set)
	//   - stack-c: stack-b
	//   - stack-d: stack-b
	// After exclude dependencies: stack-b is excluded, leaving stack-c and stack-d
	res := cli.Run("list", "--tags", "base", "--only-all-dependents", "--exclude-all-dependencies")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-c", "stack-d"),
	})
}

func TestListCombinedFiltersIncludeDependenciesWithExcludeDependents(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{

		`f:sharing.tm:` + Block("sharing_backend",
			Labels("default"),
			Expr("type", "terraform"),
			Command("terraform", "output", "-json"),
			Str("filename", "_sharing.tf"),
		).String(),
		`f:terramate.tm:` + Terramate(
			Config(
				Experiments(hcl.SharingIsCaringExperimentName),
			),
		).String(),
		`s:stack-a:id=stack-a`,
		`s:stack-b:id=stack-b`,
		`s:stack-c:id=stack-c;tags=["mid"]`,
		`s:stack-d:id=stack-d`,
		"s:stack-e",
	})

	// Create data dependencies: d→c→b→a
	s.RootEntry().CreateFile("stack-b/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_a"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-a"),
		),
	).String())
	s.RootEntry().CreateFile("stack-c/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_b"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-b"),
		),
	).String())
	s.RootEntry().CreateFile("stack-d/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_c"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-c"),
		),
	).String())

	cli := NewCLI(t, s.RootDir())

	// Select stack-c via tag, include its dependencies (a, b, c), then exclude dependents
	// After include-all-dependencies: stack-a, stack-b, stack-c
	// Dependents of those from the filtered set:
	//   - stack-a: stack-b -> exclude stack-b
	//   - stack-b: stack-c -> exclude stack-c
	//   - stack-c: none (stack-d not in filtered set)
	// After exclude dependents: only stack-a remains (it has no dependents in the final set)
	res := cli.Run("list", "--tags", "mid", "--include-all-dependencies", "--exclude-all-dependents")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-a"),
	})
}

func TestListCombinedFiltersMultipleIncludes(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		`f:sharing.tm:` + Block("sharing_backend",
			Labels("default"),
			Expr("type", "terraform"),
			Command("terraform", "output", "-json"),
			Str("filename", "_sharing.tf"),
		).String(),
		`f:terramate.tm:` + Terramate(
			Config(
				Experiments(hcl.SharingIsCaringExperimentName),
			),
		).String(),
		`s:stack-a:id=stack-a`,
		`s:stack-b:id=stack-b;tags=["middle"]`,
		`s:stack-c:id=stack-c`,
		"s:stack-x",
		"s:stack-y",
	})

	// Create data dependencies: c→b→a, y→x
	s.RootEntry().CreateFile("stack-b/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_a"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-a"),
		),
	).String())
	s.RootEntry().CreateFile("stack-c/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_b"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-b"),
		),
	).String())

	cli := NewCLI(t, s.RootDir())

	// Select stack-b via tag, include dependencies and dependents
	// This should give: stack-a (dependency), stack-b (original), stack-c (dependent)
	res := cli.Run("list", "--tags", "middle", "--include-all-dependencies", "--include-all-dependents")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-a", "stack-b", "stack-c"),
	})
}

func TestListExcludeFiltersWithNoSelection(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		`f:sharing.tm:` + Block("sharing_backend",
			Labels("default"),
			Expr("type", "terraform"),
			Command("terraform", "output", "-json"),
			Str("filename", "_sharing.tf"),
		).String(),
		`f:terramate.tm:` + Terramate(
			Config(
				Experiments(hcl.SharingIsCaringExperimentName),
			),
		).String(),
		`s:stack-a:id=stack-a`,
		`s:stack-b:id=stack-b`,
		`s:stack-c:id=stack-c`,
	})

	// Create data dependencies: c→b→a
	s.RootEntry().CreateFile("stack-b/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_a"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-a"),
		),
	).String())
	s.RootEntry().CreateFile("stack-c/inputs.tm.hcl", Doc(
		Block("input",
			Labels("dep_b"),
			Str("backend", "default"),
			Expr("value", "null"),
			Str("from_stack_id", "stack-b"),
		),
	).String())

	cli := NewCLI(t, s.RootDir())

	// With only-all-dependencies on all stacks, we get all stacks that are dependencies
	// Then exclude-all-dependents removes those that have dependents in the filtered set
	// All stacks: a, b, c
	// Dependencies: a (dep of b), b (dep of c) = a, b
	// Dependents of a in {a,b}: b -> exclude b
	// Result: a
	res := cli.Run("list", "--only-all-dependencies", "--exclude-all-dependents")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-a"),
	})
}

func TestListNativeAfterDoesNotWidenScope(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{

		`f:sharing.tm:` + Block("sharing_backend",
			Labels("default"),
			Expr("type", "terraform"),
			Command("terraform", "output", "-json"),
			Str("filename", "_sharing.tf"),
		).String(),
		`f:terramate.tm:` + Terramate(
			Config(
				Experiments(hcl.SharingIsCaringExperimentName),
			),
		).String(),
		"s:stack-a",
		`s:stack-b:after=["/stack-a"]`,
		`s:stack-c:after=["/stack-b"];tags=["leaf"]`,
	})
	s.Git().CommitAll("init stacks")
	s.Git().Push("main")
	s.Git().CheckoutNew("test-branch")

	// Change stack-c
	s.RootEntry().CreateFile("stack-c/test.txt", "change")
	s.Git().Add("stack-c/test.txt")
	s.Git().Commit("change stack-c")

	cli := NewCLI(t, s.RootDir())

	// First, check what --changed returns without any dependency flags
	res := cli.Run("list", "--changed")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-c"),
	})

	// NEGATIVE TEST: native after should NOT widen scope
	// With --include-all-dependencies on changed stack-c, we should only get stack-c itself
	// because after is for ordering only, not data dependencies
	res = cli.Run("list", "--changed", "--include-all-dependencies")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-c"),
	})

	// Also test with tag selection
	res = cli.Run("list", "--tags", "leaf", "--include-all-dependencies")
	AssertRunResult(t, res, RunExpected{
		Stdout: nljoin("stack-c"),
	})
}
