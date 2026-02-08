// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package grpc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/terramate-io/terramate/plugin"
)

func TestDiscoverInstalledFiltersGRPC(t *testing.T) {
	userDir := t.TempDir()
	pluginDir := plugin.PluginDir(userDir, "demo")
	if err := os.MkdirAll(pluginDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	binPath := filepath.Join(pluginDir, "terramate")
	if err := os.WriteFile(binPath, []byte(""), 0o700); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	manifest := plugin.Manifest{
		Name:     "demo",
		Version:  "1.0.0",
		Type:     plugin.TypeGRPC,
		Protocol: plugin.ProtocolGRPC,
		Binaries: map[plugin.BinaryKind]plugin.Binary{
			plugin.BinaryCLI: {Path: filepath.Base(binPath)},
		},
	}
	if err := plugin.SaveManifest(pluginDir, manifest); err != nil {
		t.Fatalf("save manifest: %v", err)
	}

	installed, err := DiscoverInstalled(userDir)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(installed) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(installed))
	}
	if installed[0].BinaryPath != binPath {
		t.Fatalf("unexpected binary path: %s", installed[0].BinaryPath)
	}
}

func TestDiscoverInstalledUsesProtocolMetadata(t *testing.T) {
	userDir := t.TempDir()
	pluginDir := plugin.PluginDir(userDir, "meta")
	if err := os.MkdirAll(pluginDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	binPath := filepath.Join(pluginDir, "terramate")
	if err := os.WriteFile(binPath, []byte(""), 0o700); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	manifest := plugin.Manifest{
		Name:     "meta",
		Version:  "1.0.0",
		Type:     plugin.TypeGRPC,
		Protocol: plugin.ProtocolGRPC,
		Metadata: map[string]any{
			"protocol": "grpc",
		},
		Binaries: map[plugin.BinaryKind]plugin.Binary{
			plugin.BinaryCLI: {Path: filepath.Base(binPath)},
		},
	}
	if err := plugin.SaveManifest(pluginDir, manifest); err != nil {
		t.Fatalf("save manifest: %v", err)
	}

	installed, err := DiscoverInstalled(userDir)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(installed) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(installed))
	}
}
