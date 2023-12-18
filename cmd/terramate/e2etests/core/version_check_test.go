// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"fmt"
	"strings"
	"testing"

	tfversion "github.com/hashicorp/go-version"
	"github.com/madlambda/spells/assert"
	tm "github.com/terramate-io/terramate"
	. "github.com/terramate-io/terramate/cmd/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/stack"
	"github.com/terramate-io/terramate/test/sandbox"
	"github.com/terramate-io/terramate/versions"
)

func TestVersionCheck(t *testing.T) {
	t.Parallel()

	checkedCmds := map[string]string{
		"experimental metadata":  "experimental metadata",
		"experimental globals":   "experimental globals",
		"experimental run-order": "experimental run-order",
		"experimental run-graph": "experimental run-graph",
		"generate":               "generate",
		"list":                   "list",
		"run":                    fmt.Sprintf("run %s cat %s", HelperPath, stack.DefaultFilename),
	}
	uncheckedCmds := map[string]string{
		"help":            "--help",
		"version flag":    "--version",
		"version command": "version",
	}

	run := func(t *testing.T, cmd string, version string) RunResult {
		s := sandbox.New(t)
		s.BuildTree([]string{"s:stack"})
		root := s.RootEntry()
		root.CreateConfig(fmt.Sprintf(`terramate {
			required_version = "= %s"
		}`, version))

		// required because `terramate run` requires a clean repo.
		git := s.Git()
		git.CommitAll("everything")

		cli := NewCLI(t, s.RootDir())
		return cli.Run(strings.Split(cmd, " ")...)
	}

	const (
		invalidVersion = "0.0.0"
	)

	for name, checkedCmd := range checkedCmds {
		name := fmt.Sprintf("name %s is checked", name)
		checkedCmd := checkedCmd
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			AssertRunResult(t, run(t, checkedCmd, invalidVersion), RunExpected{
				Status:      1,
				StderrRegex: string(versions.ErrCheck),
			})
		})
	}
	for name, uncheckedCmd := range uncheckedCmds {
		name := fmt.Sprintf("name %s isnt checked", name)
		uncheckedCmd := uncheckedCmd
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			AssertRunResult(t, run(t, uncheckedCmd, invalidVersion), RunExpected{
				Status:       0,
				IgnoreStdout: true,
			})
		})
	}

	cmds := map[string]string{}
	for name, cmd := range checkedCmds {
		cmds[name] = cmd
	}
	for name, cmd := range uncheckedCmds {
		cmds[name] = cmd
	}

	for name, cmd := range cmds {
		name := fmt.Sprintf("%s works with valid version", name)
		cmd := cmd
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			AssertRunResult(t, run(t, cmd, tm.Version()), RunExpected{
				Status:       0,
				IgnoreStdout: true,
				IgnoreStderr: true,
			})
		})
	}
}

func TestProvidesCorrectVersion(t *testing.T) {
	t.Parallel()

	s := sandbox.NoGit(t, false)
	cli := NewCLI(t, s.RootDir())
	want := tm.Version() + "\n"

	AssertRunResult(t, cli.Run("version"), RunExpected{
		Status: 0,
		Stdout: want,
	})
	AssertRunResult(t, cli.Run("--version"), RunExpected{
		Status: 0,
		Stdout: want,
	})
}

func TestTerramateHasValidSemver(t *testing.T) {
	t.Parallel()

	_, err := tfversion.NewSemver(tm.Version())
	assert.NoError(t, err, "terramate VERSION file has invalid version")
}
