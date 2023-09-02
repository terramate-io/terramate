// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package e2etest

import (
	"strings"
	"testing"

	"github.com/terramate-io/terramate/cmd/terramate/cli"
	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/sandbox"
	"go.lsp.dev/uri"
)

func TestSafeguardCheckRemoteNotRequiredInSomeCommands(t *testing.T) {
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
		"experimental eval 1+1",
		"experimental partial-eval 1+1",
		"experimental get-config-value global",
		"experimental generate debug",
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

func TestSafeguardCheckRemoteFailsOnRunIfRemoteMainIsOutdated(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)

	stack := s.CreateStack("stack-1")
	mainTfFile := stack.CreateFile("main.tf", "# no code")

	ts := newCLI(t, s.RootDir())

	git := s.Git()
	git.CommitAll("all")

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

func TestSafeguardCheckRemoteDisabled(t *testing.T) {
	t.Parallel()

	fileContents := "# whatever"

	setup := func(t *testing.T, env ...string) (tmcli, sandbox.FileEntry, sandbox.S) {
		t.Helper()
		s := sandbox.New(t)

		stack := s.CreateStack("stack")
		someFile := stack.CreateFile("main.tf", fileContents)

		git := s.Git()
		git.CommitAll("all")

		setupLocalMainBranchBehindOriginMain(git, func() {
			stack.CreateFile("some-new-file", "testing")
		})

		return newCLI(t, s.RootDir(), env...), someFile, s
	}

	cat := test.LookPath(t, "cat")

	t.Run("make sure setup() makes origin/main unreachable", func(t *testing.T) {
		tmcli, file, _ := setup(t)
		assertRunResult(t, tmcli.run(
			"run",
			cat,
			file.HostPath(),
		),
			runExpected{
				Status:      1,
				StderrRegex: string(cli.ErrCurrentHeadIsOutOfDate),
			})
	})

	t.Run("disable check_remote safeguard using --disable-check-git-remote", func(t *testing.T) {
		tmcli, file, _ := setup(t)
		assertRunResult(t, tmcli.run(
			"run",
			"--disable-check-git-remote",
			cat,
			file.HostPath(),
		), runExpected{Stdout: fileContents})
	})

	t.Run("disable check_remote safeguard using env vars", func(t *testing.T) {
		tmcli, file, _ := setup(t, testEnviron(t)...)
		tmcli.appendEnv = append(tmcli.appendEnv, "TM_DISABLE_CHECK_GIT_REMOTE=true")

		assertRunResult(t, tmcli.run("run", cat, file.HostPath()), runExpected{
			Stdout: fileContents,
		})
	})

	t.Run("make sure terramate.config.git.check_remote=true still checks",
		func(t *testing.T) {
			tmcli, file, s := setup(t)

			const rootConfig = "terramate.tm.hcl"
			s.RootEntry().CreateFile(rootConfig, `
			terramate {
			  config {
			    git {
			      check_remote = true
			    }
			  }
			}
		`)

			git := s.Git()
			git.Add(rootConfig)
			git.Commit("commit root config")

			assertRunResult(t, tmcli.run(
				"run",
				cat,
				file.HostPath(),
			),
				runExpected{
					Status:      1,
					StderrRegex: string(cli.ErrCurrentHeadIsOutOfDate),
				})
		})

	t.Run("disable check_remote safeguard using terramate.config.git.check_remote",
		func(t *testing.T) {
			tmcli, file, s := setup(t)

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

			git := s.Git()
			git.Add(rootConfig)
			git.Commit("commit root config")

			assertRunResult(t, tmcli.run("run", cat, file.HostPath()), runExpected{
				Stdout: fileContents,
			})
		})

	t.Run("make sure --disable-git-check-remote has precedence over config file",
		func(t *testing.T) {
			tmcli, file, s := setup(t)

			const rootConfig = "terramate.tm.hcl"
			s.RootEntry().CreateFile(rootConfig, `
			terramate {
			  config {
			    git {
			      check_remote = true
			    }
			  }
			}
		`)

			git := s.Git()
			git.Add(rootConfig)
			git.Commit("commit root config")

			assertRunResult(t, tmcli.run(
				"run",
				"--disable-check-git-remote",
				cat,
				file.HostPath(),
			), runExpected{Stdout: fileContents})
		})
}

func TestSafeguardCheckRemoteDisabledWorksWithoutNetworking(t *testing.T) {
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

func TestSafeguardCheckRemoteDisjointBranchesAreUnreachable(t *testing.T) {
	t.Parallel()
	s := sandbox.New(t)

	const (
		fileContents = "body"
	)

	stack := s.CreateStack("stack-1")
	stackFile := stack.CreateFile("main.tf", fileContents)

	git := s.Git()
	git.CommitAll("first commit")

	bare := sandbox.New(t)
	git.SetRemoteURL("origin", string(uri.File(bare.Git().BareRepoAbsPath())))

	tm := newCLI(t, s.RootDir())

	cat := test.LookPath(t, "cat")
	assertRunResult(t, tm.run(
		"run",
		cat,
		stackFile.HostPath(),
	), runExpected{
		Status:      1,
		StderrRegex: string(cli.ErrCurrentHeadIsOutOfDate),
	})
}
