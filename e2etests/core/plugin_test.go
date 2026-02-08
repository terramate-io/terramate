// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	. "github.com/terramate-io/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestPluginListEmpty(t *testing.T) {
	t.Parallel()
	s := sandbox.New(t)
	cli := NewCLI(t, s.RootDir())
	res := cli.Run("plugin", "list")
	AssertRunResult(t, res, RunExpected{
		Stdout: "No plugins installed.\n",
		Status: 0,
	})
}

func TestPluginAddFromLocalSource(t *testing.T) {
	t.Parallel()
	s := sandbox.New(t)
	cli := NewCLI(t, s.RootDir())
	sourceDir := createLocalPluginDir(t, GRPCPluginPath)

	res := cli.Run("plugin", "add", "testplugin", "--source", sourceDir)
	AssertRunResult(t, res, RunExpected{
		IgnoreStdout: true,
		Status:       0,
	})

	res = cli.Run("plugin", "list")
	AssertRunResult(t, res, RunExpected{
		StdoutRegex: "testplugin",
		Status:      0,
	})
}

func TestPluginRemove(t *testing.T) {
	t.Parallel()
	s := sandbox.New(t)
	cli := NewCLI(t, s.RootDir())
	sourceDir := createLocalPluginDir(t, GRPCPluginPath)

	AssertRunResult(t, cli.Run("plugin", "add", "testplugin", "--source", sourceDir), RunExpected{IgnoreStdout: true, Status: 0})
	AssertRunResult(t, cli.Run("plugin", "remove", "testplugin"), RunExpected{IgnoreStdout: true, Status: 0})
	AssertRunResult(t, cli.Run("plugin", "list"), RunExpected{Stdout: "No plugins installed.\n", Status: 0})
}

func TestPluginAddRemoveIdempotent(t *testing.T) {
	t.Parallel()
	s := sandbox.New(t)
	cli := NewCLI(t, s.RootDir())
	sourceDir := createLocalPluginDir(t, GRPCPluginPath)

	AssertRunResult(t, cli.Run("plugin", "add", "testplugin", "--source", sourceDir), RunExpected{IgnoreStdout: true, Status: 0})
	AssertRunResult(t, cli.Run("plugin", "add", "testplugin", "--source", sourceDir), RunExpected{IgnoreStdout: true, Status: 0})
	AssertRunResult(t, cli.Run("plugin", "remove", "testplugin"), RunExpected{IgnoreStdout: true, Status: 0})
	AssertRunResult(t, cli.Run("plugin", "remove", "testplugin"), RunExpected{IgnoreStdout: true, Status: 0})
}

func createLocalPluginDir(t *testing.T, binaryPath string) string {
	t.Helper()
	dir := test.TempDir(t)
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	copyExecutable(t, binaryPath, filepath.Join(dir, "terramate"+suffix))
	return dir
}

func copyExecutable(t *testing.T, src, dest string) {
	t.Helper()
	in, err := os.Open(src)
	if err != nil {
		t.Fatalf("open source: %v", err)
	}
	defer func() {
		_ = in.Close()
	}()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o700)
	if err != nil {
		t.Fatalf("open dest: %v", err)
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		t.Fatalf("copy: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("close dest: %v", err)
	}
}
