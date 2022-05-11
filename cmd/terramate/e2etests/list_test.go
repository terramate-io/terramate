// Copyright 2021 Mineiros GmbH
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
	"path/filepath"
	"testing"

	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestCLIList(t *testing.T) {
	type testcase struct {
		name   string
		layout []string
		want   runExpected
	}

	for _, tc := range []testcase{
		{
			name: "no stack",
		},
		{
			name: "dot directories ignored",
			layout: []string{
				"f:.stack/stack.tm:stack {}",
			},
		},
		{
			name: "dot files ignored",
			layout: []string{
				"f:stack/.stack.tm:stack {}",
			},
		},
		{
			name: "dot directories ignored in isLeaf check",
			layout: []string{
				"s:stack",
				"f:stack/.substack/stack.tm:stack {}",
			},
			want: runExpected{
				Stdout: "stack\n",
			},
		},
		{
			name: "no stack, lots of dirs",
			layout: []string{
				"d:dir1/a/b/c",
				"d:dir2/a/b/c/x/y",
				"d:last/dir",
			},
		},
		{
			name:   "single stack",
			layout: []string{"s:stack"},
			want: runExpected{
				Stdout: "stack\n",
			},
		},
		{
			name: "single stack down deep inside directories",
			layout: []string{
				"d:lots",
				"d:of",
				"d:directories",
				"d:lots/lots",
				"d:of/directories/without/any/stack",
				"d:but",
				"s:there/is/a/very/deep/hidden/stack/here",
				"d:more",
				"d:waste/directories",
			},
			want: runExpected{
				Stdout: "there/is/a/very/deep/hidden/stack/here\n",
			},
		},
		{
			name: "multiple stacks at same level",
			layout: []string{
				"s:1", "s:2", "s:3",
			},
			want: runExpected{
				Stdout: "1\n2\n3\n",
			},
		},
		{
			name: "multiple stacks at multiple levels",
			layout: []string{
				"s:1",
				"s:2",
				"s:z/a",
				"s:x/b",
				"d:not-stack",
				"d:something/else/uninportant",
				"s:3/x/y/z",
			},
			want: runExpected{
				Stdout: `1
2
3/x/y/z
x/b
z/a
`,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tc.layout)

			cli := newCLI(t, s.RootDir())
			assertRunResult(t, cli.listStacks(), tc.want)
		})
	}
}

func TestListStackWithDefinitionOnNonDefaultFilename(t *testing.T) {
	s := sandbox.New(t)
	s.BuildTree([]string{"d:stack"})
	stackDir := s.DirEntry("stack")
	stackDir.CreateFile("stack.tm", "stack {}")

	cli := newCLI(t, s.RootDir())
	assertRunResult(t, cli.listStacks(), runExpected{Stdout: "stack\n"})
}

func TestListStackWithNoTerramateBlock(t *testing.T) {
	s := sandbox.New(t)
	s.BuildTree([]string{"s:stack"})
	stack := s.StackEntry("stack")
	stack.WriteConfig(hcl.Config{
		Stack: &hcl.Stack{},
	})
	cli := newCLI(t, s.RootDir())
	assertRunResult(t, cli.listStacks(), runExpected{Stdout: "stack\n"})
}

func TestListNoSuchFile(t *testing.T) {
	notExists := test.NonExistingDir(t)
	cli := newCLI(t, notExists)

	// errors from the manager are not logged in stderr
	assertRunResult(t, cli.listStacks(), runExpected{
		Status:      1,
		StderrRegex: "no such file or directory",
	})
}

func TestListDetectChangesInSubDirOfStack(t *testing.T) {
	s := sandbox.New(t)

	stack := s.CreateStack("stack")
	subdir := stack.CreateDir("sub/dir")
	subfile := subdir.CreateFile("something.sh", "# nothing")

	cli := newCLI(t, s.RootDir())

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")
	git.CheckoutNew("change-the-stack")

	subfile.Write("# changed")
	git.Add(stack.Path())
	git.Commit("stack changed")

	want := runExpected{
		Stdout: stack.RelPath() + "\n",
	}
	assertRunResult(t, cli.listChangedStacks(), want)
}

func TestListDetectChangesInSubDirOfStackWithOtherConfigs(t *testing.T) {
	s := sandbox.New(t)

	stack := s.CreateStack("stack")
	subdir := stack.CreateDir("sub")
	subsubdir := subdir.CreateDir("dir")
	subsubfile := subsubdir.CreateFile("something.sh", "# nothing")

	subdir.CreateFile(config.DefaultFilename, `
terramate {
	
}	
`)

	cli := newCLI(t, s.RootDir())

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")
	git.CheckoutNew("change-the-stack")

	subsubfile.Write("# changed")
	git.Add(stack.Path())
	git.Commit("stack changed")

	want := runExpected{
		Stdout: stack.RelPath() + "\n",
	}
	assertRunResult(t, cli.listChangedStacks(), want)
}

func TestListChangedIgnoreDeletedStackDirectory(t *testing.T) {
	s := sandbox.New(t)

	stack := s.CreateStack("stack-old")
	cli := newCLI(t, s.RootDir())

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")
	git.CheckoutNew("deleted-stack")

	test.RemoveAll(t, stack.Path())

	git.CommitAll("removed stack")

	assertRun(t, cli.listChangedStacks())
}

func TestListChangedIgnoreDeletedNonStackDirectory(t *testing.T) {
	s := sandbox.New(t)

	s.CreateStack("stack")
	toBeDeletedDir := filepath.Join(s.RootDir(), "to-be-deleted")
	test.MkdirAll(t, toBeDeletedDir)
	test.WriteFile(t, toBeDeletedDir, "test.txt", "")
	cli := newCLI(t, s.RootDir())

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")

	git.CheckoutNew("deleted-diretory")

	test.RemoveAll(t, toBeDeletedDir)
	git.CommitAll("removed directory")

	assertRun(t, cli.listChangedStacks())
}

func TestListChangedDontIgnoreStackDeletedFiles(t *testing.T) {
	s := sandbox.New(t)

	stack := s.CreateStack("stack")
	testDir := stack.CreateDir("test")
	file := testDir.CreateFile("testfile", "")
	cli := newCLI(t, s.RootDir())

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")
	git.CheckoutNew("deleted-file")

	test.RemoveAll(t, file.Path())

	git.CommitAll("removed file")

	assertRunResult(t, cli.listChangedStacks(), runExpected{
		Stdout: stack.RelPath() + "\n",
	})
}

func TestListChangedDontIgnoreStackDeletedDirectories(t *testing.T) {
	s := sandbox.New(t)

	stack := s.CreateStack("stack")
	testDir := stack.CreateDir("test")
	testDir.CreateFile("testfile1", "")
	testDir.CreateFile("testfile2", "")
	cli := newCLI(t, s.RootDir())

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")
	git.CheckoutNew("deleted-dir")

	test.RemoveAll(t, testDir.Path())

	git.CommitAll("removed directory")

	assertRunResult(t, cli.listChangedStacks(), runExpected{
		Stdout: stack.RelPath() + "\n",
	})
}

func TestListTwiceBug(t *testing.T) {
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

	cli := newCLI(t, s.RootDir())

	wantList := stack.RelPath() + "\n"
	assertRunResult(t, cli.listChangedStacks(), runExpected{Stdout: wantList})
}
