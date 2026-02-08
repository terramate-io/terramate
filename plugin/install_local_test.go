// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDetectLocalBinariesPrefersPluginNamedBinary(t *testing.T) {
	t.Parallel()
	sourceDir := t.TempDir()
	pluginDir := t.TempDir()

	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}

	pluginBinary := filepath.Join(sourceDir, "terramate-catalyst"+suffix)
	if err := os.WriteFile(pluginBinary, []byte(""), 0o700); err != nil {
		t.Fatalf("write plugin binary: %v", err)
	}

	bins, err := detectLocalBinaries(sourceDir, pluginDir, "catalyst")
	if err != nil {
		t.Fatalf("detectLocalBinaries: %v", err)
	}
	cli, ok := bins[BinaryCLI]
	if !ok {
		t.Fatalf("expected cli binary")
	}
	if cli.Path != filepath.Base(pluginBinary) {
		t.Fatalf("unexpected cli path: %s", cli.Path)
	}
	if _, err := os.Stat(filepath.Join(pluginDir, cli.Path)); err != nil {
		t.Fatalf("expected copied cli binary: %v", err)
	}
}

func TestDetectLocalBinariesUsesDefaultNames(t *testing.T) {
	t.Parallel()
	sourceDir := t.TempDir()
	pluginDir := t.TempDir()

	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}

	defaultCLI := filepath.Join(sourceDir, "terramate"+suffix)
	defaultLS := filepath.Join(sourceDir, "terramate-ls"+suffix)
	if err := os.WriteFile(defaultCLI, []byte(""), 0o700); err != nil {
		t.Fatalf("write cli binary: %v", err)
	}
	if err := os.WriteFile(defaultLS, []byte(""), 0o700); err != nil {
		t.Fatalf("write ls binary: %v", err)
	}

	bins, err := detectLocalBinaries(sourceDir, pluginDir, "catalyst")
	if err != nil {
		t.Fatalf("detectLocalBinaries: %v", err)
	}
	if _, ok := bins[BinaryCLI]; !ok {
		t.Fatalf("expected cli binary")
	}
	if _, ok := bins[BinaryLS]; !ok {
		t.Fatalf("expected ls binary")
	}
}
