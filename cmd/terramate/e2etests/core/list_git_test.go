// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/terramate-io/terramate/cmd/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestE2EListWithGit(t *testing.T) {
	t.Parallel()

	for _, tcase := range listTestcases() {
		tc := tcase
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := sandbox.New(t)
			s.BuildTree(tc.layout)

			cli := NewCLI(t, s.RootDir())
			var args []string
			for _, filter := range tc.filterTags {
				args = append(args, "--tags", filter)
			}
			for _, filter := range tc.filterNoTags {
				args = append(args, "--no-tags", filter)
			}
			AssertRunResult(t, cli.ListStacks(args...), tc.want)
		})
	}
}

func TestE2EListWithGitSubModules(t *testing.T) {
	t.Parallel()

	for _, tcase := range listTestcases() {
		tc := tcase
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rootSandbox := sandbox.New(t)
			subSandbox := sandbox.New(t)

			subSandbox.BuildTree(tc.layout)

			if len(tc.layout) > 0 {
				subGit := subSandbox.Git()
				subGit.CommitAll("sub1 commit", true)
			}

			rootGit := rootSandbox.Git()
			rootGit.AddSubmodule("sub", subSandbox.RootDir())

			rootGit.CommitAll("add submodule")

			cli := NewCLI(t, rootSandbox.RootDir())
			var args []string
			for _, filter := range tc.filterTags {
				args = append(args, "--tags", filter)
			}
			for _, filter := range tc.filterNoTags {
				args = append(args, "--no-tags", filter)
			}
			wantStdout := []string{}
			for _, line := range strings.Split(tc.want.Stdout, "\n") {
				if line != "" {
					wantStdout = append(wantStdout, fmt.Sprintf("sub/%s", line))
				} else {
					wantStdout = append(wantStdout, "")
				}
			}
			tc.want.Stdout = strings.Join(wantStdout, "\n")
			AssertRunResult(t, cli.ListStacks(args...), tc.want)
		})
	}
}

func TestListDetectChangesInSubDirOfStack(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)

	stack := s.CreateStack("stack")
	subdir := stack.CreateDir("sub/dir")
	subfile := subdir.CreateFile("something.sh", "# nothing")

	cli := NewCLI(t, s.RootDir())

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")
	git.CheckoutNew("change-the-stack")

	subfile.Write("# changed")
	git.Add(stack.Path())
	git.Commit("stack changed")

	want := RunExpected{
		Stdout: stack.RelPath() + "\n",
	}
	AssertRunResult(t, cli.ListChangedStacks(), want)
}

func TestListDetectChangesInSubDirOfStackWithOtherConfigs(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)

	stack := s.CreateStack("stack")
	subdir := stack.CreateDir("sub")
	subsubdir := subdir.CreateDir("dir")
	subsubfile := subsubdir.CreateFile("something.sh", "# nothing")

	subdir.CreateFile("config.tm", `
terramate {
	
}	
`)

	cli := NewCLI(t, s.RootDir())

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")
	git.CheckoutNew("change-the-stack")

	subsubfile.Write("# changed")
	git.Add(stack.Path())
	git.Commit("stack changed")

	want := RunExpected{
		Stdout: stack.RelPath() + "\n",
	}
	AssertRunResult(t, cli.ListChangedStacks(), want)
}

func TestListChangedIgnoreDeletedStackDirectory(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)

	stack := s.CreateStack("stack-old")
	cli := NewCLI(t, s.RootDir())

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")
	git.CheckoutNew("deleted-stack")

	test.RemoveAll(t, stack.Path())

	git.CommitAll("removed stack")

	AssertRun(t, cli.ListChangedStacks())
}

func TestListChangedIgnoreDeletedNonStackDirectory(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)

	s.CreateStack("stack")
	toBeDeletedDir := filepath.Join(s.RootDir(), "to-be-deleted")
	test.MkdirAll(t, toBeDeletedDir)
	test.WriteFile(t, toBeDeletedDir, "test.txt", "")
	cli := NewCLI(t, s.RootDir())

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")

	git.CheckoutNew("deleted-diretory")

	test.RemoveAll(t, toBeDeletedDir)
	git.CommitAll("removed directory")

	AssertRun(t, cli.ListChangedStacks())
}

func TestListChangedDontIgnoreStackDeletedFiles(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)

	stack := s.CreateStack("stack")
	testDir := stack.CreateDir("test")
	file := testDir.CreateFile("testfile", "")
	cli := NewCLI(t, s.RootDir())

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")
	git.CheckoutNew("deleted-file")

	test.RemoveAll(t, file.HostPath())

	git.CommitAll("removed file")

	AssertRunResult(t, cli.ListChangedStacks(), RunExpected{
		Stdout: stack.RelPath() + "\n",
	})
}

func TestListChangedDontIgnoreStackDeletedDirs(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)

	stack := s.CreateStack("stack")
	toBeDeletedDir := stack.CreateDir("test1")
	deepNestedDir := toBeDeletedDir.CreateDir("test2").CreateDir("test3")
	deepNestedDir.CreateFile("testfile", "")
	cli := NewCLI(t, s.RootDir())

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")
	git.CheckoutNew("deleted-dir")

	test.RemoveAll(t, toBeDeletedDir.Path())

	git.CommitAll("removed dir")

	AssertRunResult(t, cli.ListChangedStacks(), RunExpected{
		Stdout: stack.RelPath() + "\n",
	})
}

func TestListChangedDontIgnoreStackDeletedDirectories(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)

	stack := s.CreateStack("stack")
	testDir := stack.CreateDir("test")
	testDir.CreateFile("testfile1", "")
	testDir.CreateFile("testfile2", "")
	cli := NewCLI(t, s.RootDir())

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")
	git.CheckoutNew("deleted-dir")

	test.RemoveAll(t, testDir.Path())

	git.CommitAll("removed directory")

	AssertRunResult(t, cli.ListChangedStacks(), RunExpected{
		Stdout: stack.RelPath() + "\n",
	})
}

func TestListTwiceBug(t *testing.T) {
	t.Parallel()

	const (
		mainTfFileName = "main.tf"
		modname        = "modA"
	)

	s := sandbox.New(t)

	stack := s.CreateStack("stack")
	mod1 := s.CreateModule(modname)
	mod1MainTf := mod1.CreateFile(mainTfFileName, "# module A")

	stack.CreateFile("main.tf", `
module "mod1" {
source = "%s"
}`, stack.ModSource(mod1))

	git := s.Git()
	git.CommitAll("first commit")
	git.Push("main")
	git.CheckoutNew("change-stack")

	mod1MainTf.Write("# something else")
	stack.CreateFile("test.txt", "something else")
	git.CommitAll("stack and module changed")

	cli := NewCLI(t, s.RootDir())

	wantList := stack.RelPath() + "\n"
	AssertRunResult(t, cli.ListChangedStacks(), RunExpected{Stdout: wantList})
}

func TestListChangedParsingVariablesWithOptionals(t *testing.T) {
	t.Parallel()

	// This test is to ensure we can parse Terraform code that uses
	// new features from 1.3, like variables with optionals.
	// In this case, change detection is unaffected by the new optionals feature.
	s := sandbox.New(t)

	stack := s.CreateStack("stack")
	mod1 := s.CreateModule("mod1")
	mod1MainTf := mod1.CreateFile("main.tf", "# module 1")

	stack.CreateFile("main.tf", `
variable "with_optional_attribute" {
  type = object({
    a = string                # a required attribute
    b = optional(string)      # an optional attribute
    c = optional(number, 127) # an optional attribute with a default value
  })
}

module "mod1" {
  source = "%s"
}

variable "with_optional_attribute2" {
  type = object({
    b = optional(string)
  })
}`, stack.ModSource(mod1))

	git := s.Git()
	git.CommitAll("first commit")
	git.Push("main")
	git.CheckoutNew("change-module")

	mod1MainTf.Write("# something else, changed!")
	git.CommitAll("module changed")

	cli := NewCLI(t, s.RootDir())

	wantList := stack.RelPath() + "\n"
	AssertRunResult(t, cli.ListChangedStacks(), RunExpected{Stdout: wantList})
}
