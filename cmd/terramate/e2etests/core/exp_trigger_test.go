// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/madlambda/spells/assert"
	. "github.com/terramate-io/terramate/cmd/terramate/e2etests/internal/runner"
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
	AssertRunResult(t, cli.TriggerStack("stacks/stack"), RunExpected{
		IgnoreStdout: true,
	})

	git.CommitAll("commit the trigger file")
	want := RunExpected{Stdout: "stacks/stack\n"}
	AssertRunResult(t, cli.ListChangedStacks(), want)
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
	AssertRunResult(t, cli.TriggerStack("link-to-stack"), RunExpected{
		Status:      1,
		StderrRegex: "symlinks are disallowed",
	})

	cli = NewCLI(t, s.RootDir())
	AssertRunResult(t, cli.TriggerStack("/dir/link-to-stack"), RunExpected{
		Status:      1,
		StderrRegex: "symlinks are disallowed",
	})

	cli = NewCLI(t, s.RootDir())
	AssertRunResult(t, cli.TriggerStack("/link-to-dir/stack"), RunExpected{
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
	AssertRunResult(t, cli.TriggerStack(filepath.Join(relpath, "project2-stack")),
		RunExpected{
			Status:      1,
			StderrRegex: "outside project",
		})
}

func TestListDetectAsChangedTriggeredStack(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)

	stack := s.CreateStack("stack")

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")
	git.CheckoutNew("trigger-the-stack")

	cli := NewCLI(t, s.RootDir())

	AssertRunResult(t, cli.TriggerStack("/stack"), RunExpected{
		IgnoreStdout: true,
	})

	git.CommitAll("commit the trigger file")

	want := RunExpected{
		Stdout: stack.RelPath() + "\n",
	}
	AssertRunResult(t, cli.ListChangedStacks(), want)
}

func TestRunChangedDetectionIgnoresDeletedTrigger(t *testing.T) {
	t.Parallel()

	const testfile = "testfile"

	s := sandbox.New(t)

	s.BuildTree([]string{
		"s:stack",
		fmt.Sprintf("f:stack/%s:stack\n", testfile),
	})

	cli := NewCLI(t, s.RootDir())

	AssertRunResult(t, cli.TriggerStack("/stack"), RunExpected{
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

func TestRunChangedDetectsTriggeredStack(t *testing.T) {
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

	AssertRunResult(t, cli.TriggerStack("/stack-1"), RunExpected{
		IgnoreStdout: true,
	})
	git.CommitAll("commit the trigger file for stack-1")

	AssertRunResult(t, cli.Run(
		"run",
		"--changed",
		HelperPath,
		"cat",
		testfile,
	), RunExpected{IgnoreStderr: true, Stdout: nljoin("stack-1")})

	AssertRunResult(t, cli.TriggerStack("/stack-2"), RunExpected{
		IgnoreStdout: true,
	})
	git.CommitAll("commit the trigger file for stack-2")

	AssertRunResult(t, cli.Run(
		"run",
		"--changed",
		HelperPath,
		"cat",
		testfile,
	), RunExpected{IgnoreStderr: true, Stdout: nljoin("stack-1", "stack-2")})
}

func TestRunWontDetectAsChangeDeletedTrigger(t *testing.T) {
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
	AssertRunResult(t, cli.TriggerStack("/stack-1"), RunExpected{
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
