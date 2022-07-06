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
	"testing"

	"github.com/mineiros-io/terramate/stack"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestListWatchChangedFile(t *testing.T) {
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
	s := sandbox.New(t)

	extDir := s.RootEntry().CreateDir("external")
	extFile := extDir.CreateFile("file.txt", "anything")

	s.BuildTree([]string{
		`s:stack:watch=["../../this-stack-must-never-be-visible/terramate.tm.hcl"]`,
	})

	s.LoadStack("stack")

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

func TestListWatchDirectoryFails(t *testing.T) {
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
