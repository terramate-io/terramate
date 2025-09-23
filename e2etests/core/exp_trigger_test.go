// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"fmt"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"

	"github.com/madlambda/spells/assert"
	. "github.com/terramate-io/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/stack/trigger"
	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestTriggerWorksWithRelativeStackPath(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.CreateStack("dir/stacks/stack")
	git := s.Git()
	git.CommitAll("all")
	git.Push("main")
	git.CheckoutNew("trigger-the-stack")

	// execute terramate from `dir/` directory.
	cli := NewCLI(t, filepath.Join(s.RootDir(), "dir"))
	AssertRunResult(t, cli.TriggerStack(trigger.Changed, "stacks/stack"), RunExpected{
		StdoutRegex: "Created change trigger",
	})

	git.CommitAll("commit the trigger file")
	want := RunExpected{Stdout: "stacks/stack\n"}
	AssertRunResult(t, cli.ListChangedStacks(), want)
}

func TestTriggerWorksRecursivelyFromRelativeStackPath(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.CreateStack("dir/stacks/stack1")
	s.CreateStack("dir/stacks/stack2")
	s.CreateStack("other-stack")
	git := s.Git()
	git.CommitAll("all")
	git.Push("main")
	git.CheckoutNew("trigger-the-stack")

	cli := NewCLI(t, filepath.Join(s.RootDir(), "dir"))
	AssertRunResult(t, cli.Trigger("--recursive", "--status=ok"), RunExpected{
		Status:      1,
		StderrRegex: regexp.QuoteMeta("cloud filters such as --status are incompatible with --recursive flag"),
	})

	AssertRunResult(t, cli.Trigger("--recursive", "--status=ok", "stacks"), RunExpected{
		Status:      1,
		StderrRegex: regexp.QuoteMeta("cloud filters such as --status are incompatible with --recursive flag"),
	})

	AssertRunResult(t, cli.Trigger("--changed", "stacks"), RunExpected{
		Status:      1,
		StderrRegex: regexp.QuoteMeta("path is not a stack and --recursive is not provided"),
	})

	AssertRunResult(t, cli.TriggerRecursively(trigger.Changed, "stacks"), RunExpected{
		StdoutRegex: "Created change trigger",
	})

	git.CommitAll("commit the trigger file")
	want := RunExpected{Stdout: nljoin("stacks/stack1", "stacks/stack2")}
	AssertRunResult(t, cli.ListChangedStacks(), want)
}

func TestTriggerRecursivelyFromParentStackPath(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.CreateStack("dir/stacks/stack1")
	s.CreateStack("dir/stacks/stack2")
	s.CreateStack("dir/stacks/stack1/stacka")
	s.CreateStack("other-stack")

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")
	git.CheckoutNew("trigger-the-stack")

	cli := NewCLI(t, filepath.Join(s.RootDir(), "dir"))
	AssertRunResult(t, cli.TriggerRecursively(trigger.Changed, "stacks/stack1"), RunExpected{
		IgnoreStdout: true,
	})

	git.CommitAll("commit the trigger file")
	want := RunExpected{Stdout: nljoin("stacks/stack1", "stacks/stack1/stacka")}
	AssertRunResult(t, cli.ListChangedStacks(), want)
}

func TestTriggerRecursivelyWithTags(t *testing.T) {
	t.Parallel()
	s := sandbox.New(t)
	s.BuildTree([]string{
		`s:dir/s1:tags=["tag1"]`,
		`s:dir/s1/sub1:tags=["tag3"]`,
		`s:dir/s1/sub1/subsub1:tags=["tag1", "tag2"]`,
		`s:dir/other:tags=["tag1"]`,
	})
	git := s.Git()
	git.CommitAll("all")
	git.Push("main")
	git.CheckoutNew("trigger-the-stack")

	cli := NewCLI(t, filepath.Join(s.RootDir(), "dir"))

	AssertRunResult(t, cli.Trigger("s1", "--recursive", "--tags=tag1,tag3", "--no-tags=tag2"), RunExpected{
		IgnoreStdout: true,
	})

	git.CommitAll("commit the trigger file")
	want := RunExpected{Stdout: nljoin("s1", "s1/sub1")}
	AssertRunResult(t, cli.ListChangedStacks(), want)
}

func TestTriggerCorrectErrorWhenStackDoesNotExists(t *testing.T) {
	t.Parallel()
	s := sandbox.New(t)
	cli := NewCLI(t, s.RootDir())
	AssertRunResult(t, cli.TriggerStack(trigger.Changed, "non-existent"), RunExpected{
		StderrRegex: "Error: path not found",
		Status:      1,
	})
}

func TestTriggerFailsWithSymlinksInStackPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests skipped on windows")
	}
	t.Parallel()
	s := sandbox.New(t)
	s.BuildTree([]string{
		"s:dir/stack",
		"l:dir/stack:dir/link-to-stack",
		"l:dir:link-to-dir",
	})
	git := s.Git()
	git.CommitAll("all")
	git.Push("main")
	git.CheckoutNew("trigger-the-stack")

	cli := NewCLI(t, filepath.Join(s.RootDir(), "dir"))
	AssertRunResult(t, cli.TriggerStack(trigger.Changed, "link-to-stack"), RunExpected{
		Status:      1,
		StderrRegex: "symlinks are disallowed",
	})

	cli = NewCLI(t, s.RootDir())
	AssertRunResult(t, cli.TriggerStack(trigger.Changed, "/dir/link-to-stack"), RunExpected{
		Status:      1,
		StderrRegex: "symlinks are disallowed",
	})

	cli = NewCLI(t, s.RootDir())
	AssertRunResult(t, cli.TriggerStack(trigger.Changed, "/link-to-dir/stack"), RunExpected{
		Status:      1,
		StderrRegex: "symlinks are disallowed",
	})
}

func TestTriggerMustNotTriggerStacksOutsideProject(t *testing.T) {
	t.Parallel()

	project1 := sandbox.New(t)
	project2 := sandbox.New(t)

	project1.CreateStack("project1-stack")
	project2.CreateStack("project2-stack")

	git1 := project1.Git()
	git1.CommitAll("all")
	git1.Push("main")
	git1.CheckoutNew("trigger-the-stack")

	relpath, err := filepath.Rel(project1.RootDir(), project2.RootDir())
	assert.NoError(t, err)

	cli := NewCLI(t, project1.RootDir())
	AssertRunResult(t, cli.TriggerStack(trigger.Changed, filepath.Join(relpath, "project2-stack")),
		RunExpected{
			Status:      1,
			StderrRegex: "outside project",
		})
}

func TestTriggerListsStacksAsChangedWhenTriggeredForChange(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)

	stack := s.CreateStack("stack")

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")
	git.CheckoutNew("trigger-the-stack")

	cli := NewCLI(t, s.RootDir())

	AssertRunResult(t, cli.TriggerStack(trigger.Changed, "/stack"), RunExpected{
		StdoutRegex: "Created change trigger",
	})

	git.CommitAll("commit the trigger file")

	want := RunExpected{
		Stdout: stack.RelPath() + "\n",
	}
	AssertRunResult(t, cli.ListChangedStacks(), want)
}

func TestTriggerIgnoresDeletedTriggerForChange(t *testing.T) {
	t.Parallel()

	const testfile = "testfile"

	s := sandbox.New(t)
	s.BuildTree([]string{
		"s:stack",
		fmt.Sprintf("f:stack/%s:stack\n", testfile),
	})

	cli := NewCLI(t, s.RootDir())
	AssertRunResult(t, cli.TriggerStack(trigger.Changed, "/stack"), RunExpected{
		IgnoreStdout: true,
	})

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")

	git.CheckoutNew("delete-stack-trigger")

	assertNoChanges := func() {
		t.Helper()

		AssertRunResult(t, cli.Run(
			"run",
			"--changed",
			HelperPath,
			"cat",
			testfile,
		), RunExpected{Stdout: ""})
	}

	assertNoChanges()

	triggerDir := trigger.Dir(s.RootDir())
	test.RemoveAll(t, triggerDir)

	git.CommitAll("removed trigger")
	assertNoChanges()
}

func TestTriggerIgnoresDeletedTriggerForIgnore(t *testing.T) {
	t.Parallel()

	const testfile = "testfile"

	s := sandbox.New(t)
	s.BuildTree([]string{
		"s:stack",
		fmt.Sprintf("f:stack/%s:stack\n", testfile),
	})

	cli := NewCLI(t, s.RootDir())

	AssertRunResult(t, cli.TriggerStack(trigger.Ignored, "/stack"), RunExpected{
		StdoutRegex: "Created ignore trigger",
	})

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")

	git.CheckoutNew("delete-stack-trigger")

	assertNoChanges := func() {
		t.Helper()

		AssertRunResult(t, cli.Run(
			"run",
			"--changed",
			HelperPath,
			"cat",
			testfile,
		), RunExpected{Stdout: ""})
	}

	assertNoChanges()

	triggerDir := trigger.Dir(s.RootDir())
	test.RemoveAll(t, triggerDir)

	git.CommitAll("removed trigger")

	assertNoChanges()
}

func TestTriggerDetectsChangedStacksWhenTriggeredForChange(t *testing.T) {
	t.Parallel()

	const testfile = "testfile"

	s := sandbox.New(t)
	s.BuildTree([]string{
		"s:stack-1",
		"s:stack-2",
		fmt.Sprintf("f:stack-1/%s:stack-1\n", testfile),
		fmt.Sprintf("f:stack-2/%s:stack-2\n", testfile),
	})

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")

	git.CheckoutNew("trigger-the-stack")

	cli := NewCLI(t, s.RootDir())
	AssertRunResult(t, cli.Run(
		"run",
		"--changed",
		HelperPath,
		"cat",
		testfile,
	), RunExpected{Stdout: ""})

	AssertRunResult(t, cli.TriggerStack(trigger.Changed, "/stack-1"), RunExpected{
		StdoutRegex: "Created change trigger",
	})
	git.CommitAll("commit the trigger file for stack-1")

	AssertRunResult(t, cli.Run(
		"run",
		"--quiet",
		"--changed",
		HelperPath,
		"cat",
		testfile,
	), RunExpected{Stdout: nljoin("stack-1")})

	AssertRunResult(t, cli.TriggerStack(trigger.Changed, "/stack-2"), RunExpected{
		IgnoreStdout: true,
	})
	git.CommitAll("commit the trigger file for stack-2")

	AssertRunResult(t, cli.Run(
		"run",
		"--quiet",
		"--changed",
		HelperPath,
		"cat",
		testfile,
	), RunExpected{Stdout: nljoin("stack-1", "stack-2")})
}

func TestTriggerDoNotDetectsChangedStacksWhenTriggeredForIgnore(t *testing.T) {
	t.Parallel()

	const testfile = "testfile"
	s := sandbox.New(t)
	s.BuildTree([]string{
		"s:stack-1",
		"s:stack-2",
		fmt.Sprintf("f:stack-1/%s:stack-1\n", testfile),
		fmt.Sprintf("f:stack-2/%s:stack-2\n", testfile),
	})

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")

	git.CheckoutNew("trigger-the-stack")

	// change stack-1 and stack-2
	test.WriteFile(t, s.RootDir(), "stack-1/main.tf", "# changed")
	test.WriteFile(t, s.RootDir(), "stack-2/main.tf", "# changed")
	git.CommitAll("changed stack-1 and stack-2")

	// ensure stacks are detected as changed.
	cli := NewCLI(t, s.RootDir())
	AssertRunResult(t, cli.Run(
		"--quiet",
		"run",
		"--changed",
		HelperPath,
		"cat",
		testfile,
	), RunExpected{Stdout: nljoin("stack-1", "stack-2")})

	// ignore the stack-1 stack
	AssertRunResult(t, cli.TriggerStack(trigger.Ignored, "/stack-1"), RunExpected{
		StdoutRegex: "Created ignore trigger",
	})
	git.CommitAll("commit the ignore trigger file for stack-1")

	// ONLY stack-2 must be detected as changed.
	AssertRunResult(t, cli.Run(
		"run",
		"--quiet",
		"--changed",
		HelperPath,
		"cat",
		testfile,
	), RunExpected{Stdout: nljoin("stack-2")})

	// let's also ignore stack-2
	AssertRunResult(t, cli.TriggerStack(trigger.Ignored, "/stack-2"), RunExpected{
		IgnoreStdout: true,
	})
	git.CommitAll("commit the trigger file for stack-2")

	// NO stack must be detected.
	AssertRunResult(t, cli.Run(
		"run",
		"--quiet",
		"--changed",
		HelperPath,
		"cat",
		testfile,
	), RunExpected{Stdout: ""})
}

func TestRunWontDetectAsChangeDeletedChangedTrigger(t *testing.T) {
	t.Parallel()

	const testfile = "testfile"

	s := sandbox.New(t)

	s.BuildTree([]string{
		"s:stack-1",
		"s:stack-2",
		fmt.Sprintf("f:stack-1/%s:stack-1\n", testfile),
		fmt.Sprintf("f:stack-2/%s:stack-2\n", testfile),
	})

	cli := NewCLI(t, s.RootDir())
	AssertRunResult(t, cli.TriggerStack(trigger.Changed, "/stack-1"), RunExpected{
		IgnoreStdout: true,
	})

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")

	git.CheckoutNew("delete-trigger")

	test.RemoveAll(t, trigger.Dir(s.RootDir()))
	git.CommitAll("removed trigger")

	AssertRunResult(t, cli.Run(
		"run",
		"--changed",
		HelperPath,
		"cat",
		testfile,
	), RunExpected{Stdout: ""})
}
