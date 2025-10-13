// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"testing"

	. "github.com/terramate-io/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestRunIncludeAllDependents(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		`s:stack-a:tags=["base"]`,
		`s:stack-b:after=["/stack-a"]`,
		`s:stack-c:after=["/stack-b"]`,
	})
	s.Git().CommitAll("initial commit")

	cli := NewCLI(t, s.RootDir())

	// Run on stack-a with all dependents included
	res := cli.Run("run", "--tags", "base", "--include-all-dependents", "--", "echo", "executed")
	AssertRunResult(t, res, RunExpected{
		Stdout: "executed\nexecuted\nexecuted\n",
		StderrRegexes: []string{
			`Entering stack in /stack-a`,
			`Entering stack in /stack-b`,
			`Entering stack in /stack-c`,
		},
	})
}

func TestRunOnlyAllDependents(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		`s:stack-a:tags=["base"]`,
		`s:stack-b:after=["/stack-a"]`,
		`s:stack-c:after=["/stack-b"]`,
	})
	s.Git().CommitAll("initial commit")

	cli := NewCLI(t, s.RootDir())

	// Run only on dependents of stack-a (not stack-a itself)
	res := cli.Run("run", "--tags", "base", "--only-all-dependents", "--", "echo", "executed")
	AssertRunResult(t, res, RunExpected{
		Stdout: "executed\nexecuted\n",
		StderrRegexes: []string{
			`Entering stack in /stack-b`,
			`Entering stack in /stack-c`,
		},
	})
}

func TestRunIncludeAllDependencies(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		"s:stack-a",
		`s:stack-b:after=["/stack-a"]`,
		`s:stack-c:after=["/stack-b"];tags=["leaf"]`,
	})
	s.Git().CommitAll("initial commit")

	cli := NewCLI(t, s.RootDir())

	// Run on stack-c with all dependencies included
	res := cli.Run("run", "--tags", "leaf", "--include-all-dependencies", "--", "echo", "executed")
	AssertRunResult(t, res, RunExpected{
		Stdout: "executed\nexecuted\nexecuted\n",
		StderrRegexes: []string{
			`Entering stack in /stack-a`,
			`Entering stack in /stack-b`,
			`Entering stack in /stack-c`,
		},
	})
}

func TestRunOnlyAllDependencies(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		"s:stack-a",
		`s:stack-b:after=["/stack-a"]`,
		`s:stack-c:after=["/stack-b"];tags=["leaf"]`,
	})
	s.Git().CommitAll("initial commit")

	cli := NewCLI(t, s.RootDir())

	// Run only on dependencies of stack-c (not stack-c itself)
	res := cli.Run("run", "--tags", "leaf", "--only-all-dependencies", "--", "echo", "executed")
	AssertRunResult(t, res, RunExpected{
		Stdout: "executed\nexecuted\n",
		StderrRegexes: []string{
			`Entering stack in /stack-a`,
			`Entering stack in /stack-b`,
		},
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
	s.Git().CommitAll("initial commit")

	cli := NewCLI(t, s.RootDir())

	// Run on stack-a with only direct dependents (b and c, but not d)
	res := cli.Run("run", "--tags", "base", "--include-direct-dependents", "--", "echo", "executed")
	AssertRunResult(t, res, RunExpected{
		Stdout: "executed\nexecuted\nexecuted\n",
		StderrRegexes: []string{
			`Entering stack in /stack-a`,
			`Entering stack in /stack-b`,
			`Entering stack in /stack-c`,
		},
	})
}

func TestRunWithChangedStacksAndDependents(t *testing.T) {
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

	// Run changed stacks with all dependents
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
