// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"fmt"
	"testing"

	. "github.com/terramate-io/terramate/cmd/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/project"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestListWatchChangedFile(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)

	extDir := s.RootEntry().CreateDir("external")
	extFile := extDir.CreateFile("file.txt", "anything")
	extDir.CreateFile("not-changed.txt", "anything")

	s.BuildTree([]string{
		`s:stack:watch=["/external/file.txt", "/external/not-changed.txt"]`,
	})

	stack := s.LoadStack(project.NewPath("/stack"))

	cli := NewCLI(t, s.RootDir())

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")
	git.CheckoutNew("change-the-external")

	extFile.Write("changed")
	git.CommitAll("external file changed")

	want := RunExpected{
		Stdout: stack.RelPath() + "\n",
	}
	AssertRunResult(t, cli.ListChangedStacks(), want)
}

func TestListWatchRelativeChangedFile(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)

	extDir := s.RootEntry().CreateDir("external")
	extFile := extDir.CreateFile("file.txt", "anything")

	s.BuildTree([]string{
		`s:stack:watch=["../external/file.txt"]`,
	})

	stack := s.LoadStack(project.NewPath("/stack"))

	cli := NewCLI(t, s.RootDir())

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")
	git.CheckoutNew("change-the-external")

	extFile.Write("changed")
	git.CommitAll("external file changed")

	want := RunExpected{
		Stdout: stack.RelPath() + "\n",
	}
	AssertRunResult(t, cli.ListChangedStacks(), want)
}

func TestListWatchFileOutsideProject(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)

	extDir := s.RootEntry().CreateDir("external")
	extFile := extDir.CreateFile("file.txt", "anything")

	s.BuildTree([]string{
		`s:stack:watch=["../../this-stack-must-never-be-visible/terramate.tm.hcl"]`,
	})

	cli := NewCLI(t, s.RootDir())

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")
	git.CheckoutNew("change-the-external")

	extFile.Write("changed")
	git.CommitAll("external file changed")

	want := RunExpected{
		Status:      1,
		StderrRegex: string(config.ErrStackInvalidWatch),
	}
	AssertRunResult(t, cli.ListChangedStacks(), want)
}

func TestListWatchNonExistentFile(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)

	s.BuildTree([]string{
		`s:stack:watch=["/external/non-existent.txt"]`,
	})

	cli := NewCLI(t, s.RootDir())

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")
	git.CheckoutNew("change-the-external")

	s.RootEntry().CreateFile("test.txt", "anything")
	git.CommitAll("any change")

	AssertRun(t, cli.ListChangedStacks())
}

func TestListWatchElementsWithFuncalls(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)

	extDir := s.RootEntry().CreateDir("EXTERNAL")
	extFile := extDir.CreateFile("FILE.TXT", "anything")
	extDir.CreateFile("not-changed.txt", "anything")

	stackConfig := Stack(
		Expr("watch", `[tm_upper("/external/file.txt")]`),
	)

	s.RootEntry().CreateDir("stack").CreateConfig(stackConfig.String())
	stack := s.LoadStack(project.NewPath("/stack"))

	cli := NewCLI(t, s.RootDir())

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")
	git.CheckoutNew("change-the-external")

	extFile.Write("changed")
	git.CommitAll("external file changed")

	want := RunExpected{
		Stdout: stack.RelPath() + "\n",
	}
	AssertRunResult(t, cli.ListChangedStacks(), want)
}

func TestListWatchExprWithFuncalls(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)

	extDir := s.RootEntry().CreateDir("external")
	extFile1 := extDir.CreateFile("file1.txt", "anything")
	extFile2 := extDir.CreateFile("file2.txt", "anything")
	extDir.CreateFile("unrelated.txt", "anything")
	extDir.CreateFile("deps.txt",
		fmt.Sprintf("%s\n%s", extFile1.Path(), extFile2.Path()))

	// the `watch` list comes from the `deps.txt` file.
	stackConfig := Stack(
		Expr("watch", `tm_concat(tm_split("\n", tm_file("../external/deps.txt")), [
			"/external/unrelated.txt",
	  ])`),
	)

	s.RootEntry().CreateDir("stack").CreateConfig(stackConfig.String())
	stack := s.LoadStack(project.NewPath("/stack"))

	cli := NewCLI(t, s.RootDir())

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")
	git.CheckoutNew("change-the-external")

	extFile1.Write("changed")
	git.CommitAll("external file changed")

	want := RunExpected{
		Stdout: stack.RelPath() + "\n",
	}
	AssertRunResult(t, cli.ListChangedStacks(), want)
}

func TestListWatchDirectoryFails(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)

	extDir := s.RootEntry().CreateDir("external")
	extFile := extDir.CreateFile("file.txt", "anything")

	s.BuildTree([]string{
		`s:stack:watch=["/external"]`,
	})

	cli := NewCLI(t, s.RootDir())

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")
	git.CheckoutNew("change-the-external")

	extFile.Write("changed")
	git.CommitAll("external file changed")

	want := RunExpected{
		Status:      1,
		StderrRegex: string(config.ErrStackInvalidWatch),
	}
	AssertRunResult(t, cli.ListChangedStacks(), want)
}
