// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"testing"

	. "github.com/terramate-io/terramate/e2etests/internal/runner"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestScriptRunIncludeAllDependents(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		`s:stack-a:tags=["base"]`,
		`s:stack-b:after=["/stack-a"]`,
		`s:stack-c:after=["/stack-b"]`,
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

	s.Git().CommitAll("initial commit")

	cli := NewCLI(t, s.RootDir())

	// Run script on stack-a with all dependents included
	res := cli.Run("script", "run", "--tags", "base", "--include-all-dependents", "test")
	AssertRunResult(t, res, RunExpected{
		StdoutRegexes: []string{
			`executed from.*stack-a`,
			`executed from.*stack-b`,
			`executed from.*stack-c`,
		},
		IgnoreStderr: true,
	})
}

func TestScriptRunOnlyAllDependents(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		`s:stack-a:tags=["base"]`,
		`s:stack-b:after=["/stack-a"]`,
		`s:stack-c:after=["/stack-b"]`,
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

	s.Git().CommitAll("initial commit")

	cli := NewCLI(t, s.RootDir())

	// Run script only on dependents of stack-a (not stack-a itself)
	res := cli.Run("script", "run", "--tags", "base", "--only-all-dependents", "test")
	AssertRunResult(t, res, RunExpected{
		StdoutRegexes: []string{
			`executed from.*stack-b`,
			`executed from.*stack-c`,
		},
		IgnoreStderr: true,
	})
}

func TestScriptRunIncludeAllDependencies(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		"s:stack-a",
		`s:stack-b:after=["/stack-a"]`,
		`s:stack-c:after=["/stack-b"];tags=["leaf"]`,
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

	s.Git().CommitAll("initial commit")

	cli := NewCLI(t, s.RootDir())

	// Run script on stack-c with all dependencies included
	res := cli.Run("script", "run", "--tags", "leaf", "--include-all-dependencies", "test")
	AssertRunResult(t, res, RunExpected{
		StdoutRegexes: []string{
			`executed from.*stack-a`,
			`executed from.*stack-b`,
			`executed from.*stack-c`,
		},
		IgnoreStderr: true,
	})
}

func TestScriptRunOnlyAllDependencies(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		"s:stack-a",
		`s:stack-b:after=["/stack-a"]`,
		`s:stack-c:after=["/stack-b"];tags=["leaf"]`,
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

	s.Git().CommitAll("initial commit")

	cli := NewCLI(t, s.RootDir())

	// Run script only on dependencies of stack-c (not stack-c itself)
	res := cli.Run("script", "run", "--tags", "leaf", "--only-all-dependencies", "test")
	AssertRunResult(t, res, RunExpected{
		StdoutRegexes: []string{
			`executed from.*stack-a`,
			`executed from.*stack-b`,
		},
		IgnoreStderr: true,
	})
}

func TestScriptRunIncludeDirectDependents(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		`s:stack-a:tags=["base"]`,
		`s:stack-b:after=["/stack-a"]`,
		`s:stack-c:after=["/stack-a"]`,
		`s:stack-d:after=["/stack-b"]`,
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

	s.Git().CommitAll("initial commit")

	cli := NewCLI(t, s.RootDir())

	// Run script on stack-a with only direct dependents (b and c, but not d)
	res := cli.Run("script", "run", "--tags", "base", "--include-direct-dependents", "test")
	AssertRunResult(t, res, RunExpected{
		StdoutRegexes: []string{
			`executed from.*stack-a`,
			`executed from.*stack-b`,
			`executed from.*stack-c`,
		},
		IgnoreStderr: true,
	})
}

func TestScriptRunWithChangedStacksAndDependents(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		`s:stack-a:tags=["changed-test"]`,
		`s:stack-b:after=["/stack-a"]`,
		`s:stack-c:after=["/stack-b"]`,
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

	s.Git().CommitAll("initial commit")
	s.Git().Push("main")
	s.Git().CheckoutNew("test-branch")

	// Make a change to stack-a
	s.RootEntry().CreateFile("stack-a/test.txt", "changed")
	s.Git().Add("stack-a/test.txt")
	s.Git().Commit("change stack-a")

	cli := NewCLI(t, s.RootDir())

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
