// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"strings"
	"testing"

	"github.com/terramate-io/terramate/cmd/terramate/cli"
	"github.com/terramate-io/terramate/cmd/terramate/cli/clitest"
	. "github.com/terramate-io/terramate/cmd/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/test/sandbox"
	"go.lsp.dev/uri"
)

func TestSafeguardsUsages(t *testing.T) {
	t.Parallel()

	setup := func(t *testing.T, fileContent string, env ...string) (CLI, sandbox.FileEntry, sandbox.S) {
		t.Helper()
		s := sandbox.New(t)

		stack := s.CreateStack("stack")
		someFile := stack.CreateFile("main.tf", fileContent)

		git := s.Git()
		git.CommitAll("all")

		// make the "untracked" file.
		_ = stack.CreateFile("untracked.tf", "# something")

		// make it out of sync
		setupLocalMainBranchBehindOriginMain(git, func() {
			stack.CreateFile("some-new-file", "testing")
		})

		return NewCLI(t, s.RootDir(), env...), someFile, s
	}

	t.Run("multiple keywords can be provided to --disable-safeguards=<options>", func(t *testing.T) {
		const fileContents = "# something"
		tmcli, file, _ := setup(t, fileContents)
		AssertRunResult(t, tmcli.Run(
			"run",
			"--quiet",
			"--disable-safeguards=git-untracked,git-out-of-sync",
			HelperPath,
			"cat",
			file.HostPath(),
		), RunExpected{Stdout: fileContents})
	})

	t.Run("--disable-safeguards=<options> can be provided multiple times", func(t *testing.T) {
		const fileContents = "# something"
		tmcli, file, _ := setup(t, fileContents)
		AssertRunResult(t, tmcli.Run(
			"run",
			"--quiet",
			"--disable-safeguards=git-untracked",
			"--disable-safeguards=git-out-of-sync",
			HelperPath,
			"cat",
			file.HostPath(),
		), RunExpected{Stdout: fileContents})
	})

	t.Run("-X conflicts with --disable-safeguards=none", func(t *testing.T) {
		const fileContents = "# something"
		tmcli, file, _ := setup(t, fileContents)
		AssertRunResult(t, tmcli.Run(
			"run",
			"--quiet",
			"--disable-safeguards=none",
			"-X",
			HelperPath,
			"cat",
			file.HostPath(),
		), RunExpected{
			Status:      1,
			StderrRegex: string(clitest.ErrSafeguardKeywordValidation),
		})
	})

	t.Run("--disable-safeguards=none,all conflicts", func(t *testing.T) {
		const fileContents = "# something"
		tmcli, file, _ := setup(t, fileContents)
		AssertRunResult(t, tmcli.Run(
			"run",
			"--quiet",
			"--disable-safeguards=all,none",
			HelperPath,
			"cat",
			file.HostPath(),
		), RunExpected{
			Status:      1,
			StderrRegex: string(clitest.ErrSafeguardKeywordValidation),
		})
	})

	t.Run("TM_DISABLE_SAFEGUARDS=none,all conflicts", func(t *testing.T) {
		const fileContents = "# something"
		tmcli, file, _ := setup(t, fileContents)
		tmcli.AppendEnv = append(tmcli.AppendEnv, "M_DISABLE_SAFEGUARDS=none,all")
		AssertRunResult(t, tmcli.Run(
			"run",
			"--quiet",
			"--disable-safeguards=all,none",
			HelperPath,
			"cat",
			file.HostPath(),
		), RunExpected{
			Status:      1,
			StderrRegex: string(clitest.ErrSafeguardKeywordValidation),
		})
	})
}

func TestSafeguardCheckRemoteNotRequiredInSomeCommands(t *testing.T) {
	t.Parallel()

	// Regression test to guarantee that all git checks
	// are disabled and no git operation will be performed for certain commands.
	// Some people like to get some coding done on airplanes :-)
	s := sandbox.New(t)
	git := s.Git()
	git.SetRemoteURL("origin", "http://non-existant/terramate.git")

	s.CreateStack("stack")

	cli := NewCLI(t, s.RootDir())

	cmds := []string{
		"debug show metadata",
		"debug show globals",
		"experimental run-order",
		"experimental run-graph",
		"experimental eval 1+1",
		"experimental partial-eval 1+1",
		"experimental get-config-value global",
		"debug show generate-origins",
		"create stack-2",
		"generate",
		"list",
	}
	for _, cmd := range cmds {
		t.Run(cmd, func(t *testing.T) {
			args := strings.Split(cmd, " ")
			AssertRunResult(t, cli.Run(args...), RunExpected{
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

	ts := NewCLI(t, s.RootDir())

	git := s.Git()
	git.CommitAll("all")

	setupLocalMainBranchBehindOriginMain(git, func() {
		stack.CreateFile("tempfile", "any content")
	})

	testrun := func() {
		wantRes := RunExpected{
			Status:      1,
			StderrRegex: string(cli.ErrCurrentHeadIsOutOfDate),
		}

		AssertRunResult(t, ts.Run(
			"run",
			HelperPath,
			"cat",
			mainTfFile.HostPath(),
		), wantRes)

		AssertRunResult(t, ts.Run(
			"run",
			"--changed",
			HelperPath,
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

	setup := func(t *testing.T, env ...string) (CLI, sandbox.FileEntry, sandbox.S) {
		t.Helper()
		s := sandbox.New(t)

		stack := s.CreateStack("stack")
		someFile := stack.CreateFile("main.tf", fileContents)

		git := s.Git()
		git.CommitAll("all")

		setupLocalMainBranchBehindOriginMain(git, func() {
			stack.CreateFile("some-new-file", "testing")
		})

		return NewCLI(t, s.RootDir(), env...), someFile, s
	}

	t.Run("make sure setup() makes origin/main unreachable", func(t *testing.T) {
		tmcli, file, _ := setup(t)
		AssertRunResult(t, tmcli.Run(
			"run",
			HelperPath,
			"cat",
			file.HostPath(),
		), RunExpected{
			Status:      1,
			StderrRegex: string(cli.ErrCurrentHeadIsOutOfDate),
		})
	})

	t.Run("disable check_remote safeguard using --disable-check-git-remote", func(t *testing.T) {
		tmcli, file, _ := setup(t)
		AssertRunResult(t, tmcli.Run(
			"run",
			"--quiet",
			"--disable-check-git-remote",
			HelperPath,
			"cat",
			file.HostPath(),
		), RunExpected{Stdout: fileContents})
	})

	t.Run("disable check_remote safeguard using --disable-safeguards=git-out-of-sync", func(t *testing.T) {
		tmcli, file, _ := setup(t)
		AssertRunResult(t, tmcli.Run(
			"run",
			"--quiet",
			"--disable-safeguards=git-out-of-sync",
			HelperPath,
			"cat",
			file.HostPath(),
		), RunExpected{Stdout: fileContents})
	})

	t.Run("disable check_remote safeguard using --disable-safeguards=git", func(t *testing.T) {
		tmcli, file, _ := setup(t)
		AssertRunResult(t, tmcli.Run(
			"run",
			"--quiet",
			"--disable-safeguards=git",
			HelperPath,
			"cat",
			file.HostPath(),
		), RunExpected{Stdout: fileContents})
	})

	t.Run("disable check_remote safeguard using -X", func(t *testing.T) {
		tmcli, file, _ := setup(t)
		AssertRunResult(t, tmcli.Run(
			"run",
			"--quiet",
			"-X",
			HelperPath,
			"cat",
			file.HostPath(),
		), RunExpected{Stdout: fileContents})
	})

	t.Run("disable check_remote safeguard using env=true", func(t *testing.T) {
		tmcli, file, _ := setup(t, testEnviron(t)...)
		tmcli.AppendEnv = append(tmcli.AppendEnv, "TM_DISABLE_CHECK_GIT_REMOTE=true")

		AssertRunResult(t, tmcli.Run("run", "--quiet", HelperPath,
			"cat", file.HostPath()), RunExpected{
			Stdout: fileContents,
		})
	})

	t.Run("disable check_remote safeguard using env=1", func(t *testing.T) {
		tmcli, file, _ := setup(t, testEnviron(t)...)
		tmcli.AppendEnv = append(tmcli.AppendEnv, "TM_DISABLE_CHECK_GIT_REMOTE=1")

		AssertRunResult(t, tmcli.Run("run", "--quiet", HelperPath,
			"cat", file.HostPath()), RunExpected{
			Stdout: fileContents,
		})
	})

	t.Run("disable check_remote safeguard using env TM_DISABLE_SAFEGUARDS=git-out-of-sync", func(t *testing.T) {
		tmcli, file, _ := setup(t, testEnviron(t)...)
		tmcli.AppendEnv = append(tmcli.AppendEnv, "TM_DISABLE_SAFEGUARDS=git-out-of-sync")

		AssertRunResult(t, tmcli.Run("run", "--quiet", HelperPath,
			"cat", file.HostPath()), RunExpected{
			Stdout: fileContents,
		})
	})

	t.Run("disable check_remote safeguard using env TM_DISABLE_SAFEGUARDS=git", func(t *testing.T) {
		tmcli, file, _ := setup(t, testEnviron(t)...)
		tmcli.AppendEnv = append(tmcli.AppendEnv, "TM_DISABLE_SAFEGUARDS=git")

		AssertRunResult(t, tmcli.Run("run", "--quiet", HelperPath,
			"cat", file.HostPath()), RunExpected{
			Stdout: fileContents,
		})
	})

	t.Run("safeguard explicitly enabled by config terramate.config.git.check_remote=true",
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

			AssertRunResult(t, tmcli.Run(
				"run",
				HelperPath,
				"cat",
				file.HostPath(),
			),
				RunExpected{
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

			AssertRunResult(t, tmcli.Run("run", "--quiet", HelperPath,
				"cat", file.HostPath()), RunExpected{
				Stdout: fileContents,
			})
		})

	t.Run("disable check_remote safeguard using terramate.config.disable_safeguards",
		func(t *testing.T) {
			tmcli, file, s := setup(t)

			const rootConfig = "terramate.tm.hcl"
			s.RootEntry().CreateFile(rootConfig, `
			terramate {
			  config {
			    disable_safeguards = ["git-out-of-sync"]
			  }
			}
		`)

			git := s.Git()
			git.Add(rootConfig)
			git.Commit("commit root config")

			AssertRunResult(t, tmcli.Run("run", "--quiet", HelperPath,
				"cat", file.HostPath()), RunExpected{
				Stdout: fileContents,
			})
		})

	t.Run("safeguards re-enabled by --disable-safeguards=none",
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

			AssertRunResult(t, tmcli.Run(
				"run",
				"--disable-safeguards=none",
				HelperPath,
				"cat",
				file.HostPath(),
			),
				RunExpected{
					Status:      1,
					StderrRegex: string(cli.ErrCurrentHeadIsOutOfDate),
				})
		})

	t.Run("re-enabled from config.disable_safegaurds by --disable-safeguards=none",
		func(t *testing.T) {
			tmcli, file, s := setup(t)

			const rootConfig = "terramate.tm.hcl"
			s.RootEntry().CreateFile(rootConfig, `
			terramate {
			  config {
			    disable_safeguards = ["git-out-of-sync"]
			  }
			}
		`)

			git := s.Git()
			git.Add(rootConfig)
			git.Commit("commit root config")

			AssertRunResult(t, tmcli.Run(
				"run",
				"--disable-safeguards=none",
				HelperPath,
				"cat",
				file.HostPath(),
			), RunExpected{
				Status:      1,
				StderrRegex: string(cli.ErrCurrentHeadIsOutOfDate),
			})
		})

	t.Run("safeguards re-enabled by TM_DISABLE_SAFEGUARDS=none",
		func(t *testing.T) {
			tmcli, file, s := setup(t)
			tmcli.AppendEnv = append(tmcli.AppendEnv, "TM_DISABLE_SAFEGUARDS=none")

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

			AssertRunResult(t, tmcli.Run(
				"run",
				HelperPath,
				"cat",
				file.HostPath(),
			),
				RunExpected{
					Status:      1,
					StderrRegex: string(cli.ErrCurrentHeadIsOutOfDate),
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

			AssertRunResult(t, tmcli.Run(
				"run",
				"--quiet",
				"--disable-check-git-remote",
				HelperPath,
				"cat",
				file.HostPath(),
			), RunExpected{Stdout: fileContents})
		})

	t.Run("make sure --disable-safeguards=git-out-of-sync has precedence over config file",
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

			AssertRunResult(t, tmcli.Run(
				"run",
				"--quiet",
				"--disable-safeguards=git-out-of-sync",
				HelperPath,
				"cat",
				file.HostPath(),
			), RunExpected{Stdout: fileContents})
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

	tm := NewCLI(t, s.RootDir())

	AssertRunResult(t, tm.Run(
		"run",
		HelperPath,
		"cat",
		stackFile.HostPath(),
	), RunExpected{
		Status:      1,
		StderrRegex: "Could not resolve host: non-existent",
	})
	AssertRunResult(t, tm.Run(
		"run",
		"--quiet",
		"--disable-check-git-remote",
		HelperPath,
		"cat",
		stackFile.HostPath(),
	), RunExpected{
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

	tm := NewCLI(t, s.RootDir())

	AssertRunResult(t, tm.Run(
		"run",
		HelperPath,
		"cat",
		stackFile.HostPath(),
	), RunExpected{
		Status:      1,
		StderrRegex: string(cli.ErrCurrentHeadIsOutOfDate),
	})
}
