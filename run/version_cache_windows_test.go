//go:build windows

// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package run

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeCmdScript(t *testing.T, path string, counter string, versionLine string) {
	t.Helper()
	// .cmd script that prints version if --version and appends to counter
	// Use proper quoting for the counter path
	content := "@echo off\r\n" +
		"if \"%~1\"==\"--version\" (\r\n" +
		"  echo " + versionLine + "\r\n" +
		")\r\n" +
		"echo x>>\"" + strings.ReplaceAll(counter, "\"", "\"\"") + "\"\r\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}
}

func TestResolveVersionCachesPerResolvedPath_Windows(t *testing.T) {
	ResetVersionCache()

	tdir := t.TempDir()
	counter := filepath.Join(tdir, "count.txt")
	bin := filepath.Join(tdir, "foo.cmd")

	writeCmdScript(t, bin, counter, "foo v1.2.3")

	env := []string{"PATH=" + tdir}
	ctx := context.Background()

	if v := ResolveVersion(ctx, env, "foo"); v == "" {
		t.Fatalf("expected version, got empty")
	}
	_ = ResolveVersion(ctx, env, "foo")
	_ = ResolveVersion(ctx, env, "foo")

	b, err := os.ReadFile(counter)
	if err != nil {
		t.Fatalf("read counter: %v", err)
	}
	if got := strings.Count(string(b), "x"); got != 1 {
		t.Fatalf("expected one invocation, got %d", got)
	}
}

func TestResolveVersionCacheKeyIsResolvedPath_Windows(t *testing.T) {
	ResetVersionCache()

	d1 := t.TempDir()
	d2 := t.TempDir()
	c1 := filepath.Join(d1, "count.txt")
	c2 := filepath.Join(d2, "count.txt")

	writeCmdScript(t, filepath.Join(d1, "foo.cmd"), c1, "foo v1.2.3")
	writeCmdScript(t, filepath.Join(d2, "foo.cmd"), c2, "foo v1.2.3")

	ctx := context.Background()
	_ = ResolveVersion(ctx, []string{"PATH=" + d1}, "foo")
	_ = ResolveVersion(ctx, []string{"PATH=" + d2}, "foo")

	b1, err := os.ReadFile(c1)
	if err != nil {
		t.Fatalf("read counter1: %v", err)
	}
	b2, err := os.ReadFile(c2)
	if err != nil {
		t.Fatalf("read counter2: %v", err)
	}
	if strings.Count(string(b1), "x") != 1 || strings.Count(string(b2), "x") != 1 {
		t.Fatalf("expected one invocation per resolved path")
	}
}

func TestSetTestVersionOverrideSkipsShellOut_Windows(t *testing.T) {
	ResetVersionCache()
	SetTestVersionOverride("foo", "9.9.9")
	t.Cleanup(func() { SetTestVersionOverride("foo", "") })

	ctx := context.Background()
	if v := ResolveVersion(ctx, []string{"PATH=C:\\nonexistent"}, "foo"); v != "9.9.9" {
		t.Fatalf("expected override version, got %q", v)
	}
}

func TestSeedVersionForResolvedPathSkipsShellOut_Windows(t *testing.T) {
	ResetVersionCache()

	d := t.TempDir()
	counter := filepath.Join(d, "count.txt")
	bin := filepath.Join(d, "foo.cmd")
	writeCmdScript(t, bin, counter, "foo v1.2.3")

	resolved, err := LookPath("foo", []string{"PATH=" + d})
	if err != nil {
		t.Fatalf("lookpath: %v", err)
	}
	SeedVersionForResolvedPath(resolved, "2.2.2")

	ctx := context.Background()
	if v := ResolveVersion(ctx, []string{"PATH=" + d}, "foo"); v != "2.2.2" {
		t.Fatalf("expected seeded version, got %q", v)
	}

	if _, err := os.Stat(counter); err == nil {
		t.Fatalf("expected no shell-out (counter file should not exist)")
	}
}
