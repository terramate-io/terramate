// Copyright 2022 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package e2etest

import (
	"fmt"
	"testing"

	"github.com/mineiros-io/terramate/stack"
	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
	"github.com/mineiros-io/terramate/test/sandbox"
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

	stack := s.LoadStack("stack")

	cli := newCLI(t, s.RootDir())

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")
	git.CheckoutNew("change-the-external")

	extFile.Write("changed")
	git.CommitAll("external file changed")

	want := runExpected{
		Stdout: stack.RelPath() + "\n",
	}
	assertRunResult(t, cli.listChangedStacks(), want)
}

func TestListWatchRelativeChangedFile(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)

	extDir := s.RootEntry().CreateDir("external")
	extFile := extDir.CreateFile("file.txt", "anything")

	s.BuildTree([]string{
		`s:stack:watch=["../external/file.txt"]`,
	})

	stack := s.LoadStack("stack")

	cli := newCLI(t, s.RootDir())

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")
	git.CheckoutNew("change-the-external")

	extFile.Write("changed")
	git.CommitAll("external file changed")

	want := runExpected{
		Stdout: stack.RelPath() + "\n",
	}
	assertRunResult(t, cli.listChangedStacks(), want)
}

func TestListWatchFileOutsideProject(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)

	extDir := s.RootEntry().CreateDir("external")
	extFile := extDir.CreateFile("file.txt", "anything")

	s.BuildTree([]string{
		`s:stack:watch=["../../this-stack-must-never-be-visible/terramate.tm.hcl"]`,
	})

	cli := newCLI(t, s.RootDir())

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")
	git.CheckoutNew("change-the-external")

	extFile.Write("changed")
	git.CommitAll("external file changed")

	want := runExpected{
		Status:      1,
		StderrRegex: string(stack.ErrInvalidWatch),
	}
	assertRunResult(t, cli.listChangedStacks(), want)
}

func TestListWatchNonExistentFile(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)

	s.BuildTree([]string{
		`s:stack:watch=["/external/non-existent.txt"]`,
	})

	cli := newCLI(t, s.RootDir())

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")
	git.CheckoutNew("change-the-external")

	s.RootEntry().CreateFile("test.txt", "anything")
	git.CommitAll("any change")

	assertRun(t, cli.listChangedStacks())
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
	stack := s.LoadStack("stack")

	cli := newCLI(t, s.RootDir())

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")
	git.CheckoutNew("change-the-external")

	extFile.Write("changed")
	git.CommitAll("external file changed")

	want := runExpected{
		Stdout: stack.RelPath() + "\n",
	}
	assertRunResult(t, cli.listChangedStacks(), want)
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
	stack := s.LoadStack("stack")

	cli := newCLI(t, s.RootDir())

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")
	git.CheckoutNew("change-the-external")

	extFile1.Write("changed")
	git.CommitAll("external file changed")

	want := runExpected{
		Stdout: stack.RelPath() + "\n",
	}
	assertRunResult(t, cli.listChangedStacks(), want)
}

func TestListWatchDirectoryFails(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)

	extDir := s.RootEntry().CreateDir("external")
	extFile := extDir.CreateFile("file.txt", "anything")

	s.BuildTree([]string{
		`s:stack:watch=["/external"]`,
	})

	cli := newCLI(t, s.RootDir())

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")
	git.CheckoutNew("change-the-external")

	extFile.Write("changed")
	git.CommitAll("external file changed")

	want := runExpected{
		Status:      1,
		StderrRegex: string(stack.ErrInvalidWatch),
	}
	assertRunResult(t, cli.listChangedStacks(), want)
}
