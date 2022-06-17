package e2etest

import (
	"path/filepath"
	"testing"

	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestE2EListWithGit(t *testing.T) {
	for _, tc := range listTestcases() {
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tc.layout)

			cli := newCLI(t, s.RootDir())
			assertRunResult(t, cli.listStacks(), tc.want)
		})
	}
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
