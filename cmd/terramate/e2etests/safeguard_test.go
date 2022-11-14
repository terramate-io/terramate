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
	"strings"
	"testing"

	"github.com/mineiros-io/terramate/cmd/terramate/cli"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestSafeguardNotRequiredInSomeCommands(t *testing.T) {
	t.Parallel()

	// Regression test to guarantee that all git checks
	// are disabled and no git operation will be performed for certain commands.
	// Some people like to get some coding done on airplanes :-)
	s := sandbox.New(t)
	git := s.Git()
	git.SetRemoteURL("origin", "http://non-existant/terramate.git")

	s.CreateStack("stack")

	cli := newCLI(t, s.RootDir())

	cmds := []string{
		"experimental metadata",
		"experimental globals",
		"experimental run-order",
		"experimental run-graph",
		"create stack-2",
		"generate",
		"list",
	}
	for _, cmd := range cmds {
		t.Run(cmd, func(t *testing.T) {
			args := strings.Split(cmd, " ")
			assertRunResult(t, cli.run(args...), runExpected{
				IgnoreStdout: true,
			})
		})
	}
}

func TestSafeguardFailsOnRunIfRemoteMainIsOutdated(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)

	stack := s.CreateStack("stack-1")
	mainTfFile := stack.CreateFile("main.tf", "# no code")

	ts := newCLI(t, s.RootDir())

	git := s.Git()

	git.Add(".")
	git.Commit("all")

	setupLocalMainBranchBehindOriginMain(git, func() {
		stack.CreateFile("tempfile", "any content")
	})

	testrun := func() {
		wantRes := runExpected{
			Status:      1,
			StderrRegex: string(cli.ErrCurrentHeadIsOutOfDate),
		}

		assertRunResult(t, ts.run(
			"run",
			testHelperBin,
			"cat",
			mainTfFile.HostPath(),
		), wantRes)

		assertRunResult(t, ts.run(
			"run",
			"--changed",
			testHelperBin,
			"cat",
			mainTfFile.HostPath(),
		), wantRes)
	}

	testrun()

	git.CheckoutNew("branch")

	// we create two commits so we can also test from a DETACHED HEAD.
	stack.CreateFile("tempfile2", "any content")
	git.CommitAll("add tempfile2")

	stack.CreateFile("tempfile3", "any content")
	git.CommitAll("add tempfile3")

	testrun()

	git.Checkout("HEAD^1")

	testrun()
}

func TestSafeguardDisableGitCheckRemote(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)

	stack := s.CreateStack("stack")
	fileContents := "# whatever"
	someFile := stack.CreateFile("main.tf", fileContents)

	tmcli := newCLI(t, s.RootDir())

	git := s.Git()

	git.Add(".")
	git.Commit("all")

	setupLocalMainBranchBehindOriginMain(git, func() {
		stack.CreateFile("some-new-file", "testing")
	})

	cat := test.LookPath(t, "cat")

	t.Run("check remote is not reachable", func(t *testing.T) {
		assertRunResult(t, tmcli.run(
			"run",
			cat,
			someFile.HostPath(),
		),
			runExpected{
				Status:      1,
				StderrRegex: string(cli.ErrCurrentHeadIsOutOfDate),
			})
	})

	t.Run("disable check using cmd args", func(t *testing.T) {
		assertRunResult(t, tmcli.run(
			"run",
			"--disable-check-git-remote",
			cat,
			someFile.HostPath(),
		), runExpected{Stdout: fileContents})
	})

	t.Run("disable check using env vars", func(t *testing.T) {
		ts := newCLI(t, s.RootDir())
		ts.env = append([]string{
			"TM_DISABLE_CHECK_GIT_REMOTE=true",
		}, testEnviron()...)

		assertRunResult(t, ts.run("run", cat, someFile.HostPath()), runExpected{
			Stdout: fileContents,
		})
	})

	t.Run("disable check using hcl config", func(t *testing.T) {
		const rootConfig = "terramate.tm.hcl"

		s.RootEntry().CreateFile(rootConfig, `
			terramate {
			  config {
			    git {
			      check_remote = false
			    }
			  }
			}
		`)
		defer s.RootEntry().RemoveFile(rootConfig)

		git.Add(rootConfig)
		git.Commit("commit root config")

		assertRunResult(t, tmcli.run("run", cat, someFile.HostPath()), runExpected{
			Stdout: fileContents,
		})
	})
}

func TestSafeguardWithDisabledCheckRemoteFromConfig(t *testing.T) {
	t.Parallel()

	const rootConfig = "terramate.tm.hcl"

	s := sandbox.New(t)

	stack := s.CreateStack("stack")
	fileContents := "# whatever"
	someFile := stack.CreateFile("main.tf", fileContents)

	cat := test.LookPath(t, "cat")

	s.RootEntry().CreateFile(rootConfig, `
			terramate {
			  config {
			    git {
			      check_remote = false
			    }
			  }
			}
		`)

	git := s.Git()

	git.Add(".")
	git.Commit("all")

	tmcli := newCLI(t, s.RootDir())
	assertRunResult(t, tmcli.run("run",
		cat, someFile.HostPath()), runExpected{
		Stdout: fileContents,
	})
	assertRunResult(t, tmcli.run("run", "--changed",
		cat, someFile.HostPath()), runExpected{
		Stdout: fileContents,
	})

	git.Push("main")
	assertRunResult(t, tmcli.run("run",
		cat, someFile.HostPath()), runExpected{
		Stdout: fileContents,
	})
	// baseref=HEAD^
	assertRunResult(t, tmcli.run("run", "--changed",
		cat, someFile.HostPath()), runExpected{
		Stdout: fileContents,
	})

	git.CheckoutNew("test")
	assertRun(t, tmcli.run("run", "--changed",
		cat, someFile.HostPath()))
}

func TestSafeguardRunWithGitRemoteCheckDisabledWorksWithoutNetworking(t *testing.T) {
	t.Parallel()

	// Regression test to guarantee that all git checks
	// are disabled and no git operation will be performed on this case.
	// So running terramate run --disable-check-git-remote will
	// not fail if there is no networking.
	// Some people like to get some coding done on airplanes :-)
	const (
		fileContents   = "body"
		nonExistentGit = "http://non-existent/terramate.git"
	)

	s := sandbox.New(t)

	stack := s.CreateStack("stack-1")
	stackFile := stack.CreateFile("main.tf", fileContents)

	git := s.Git()
	git.Add(".")
	git.CommitAll("first commit")

	git.SetRemoteURL("origin", nonExistentGit)

	tm := newCLI(t, s.RootDir())

	cat := test.LookPath(t, "cat")
	assertRunResult(t, tm.run(
		"run",
		cat,
		stackFile.HostPath(),
	), runExpected{
		Status:      1,
		StderrRegex: "Could not resolve host: non-existent",
	})
	assertRunResult(t, tm.run(
		"run",
		"--disable-check-git-remote",
		cat,
		stackFile.HostPath(),
	), runExpected{
		Stdout: fileContents,
	})
}
