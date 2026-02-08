// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListInstalledEmpty(t *testing.T) {
	t.Parallel()
	userDir := t.TempDir()
	plugins, err := ListInstalled(userDir)
	if err != nil {
		t.Fatalf("ListInstalled: %v", err)
	}
	if len(plugins) != 0 {
		t.Fatalf("expected empty list, got %d", len(plugins))
	}
}

func TestListInstalledWithPlugins(t *testing.T) {
	t.Parallel()
	userDir := t.TempDir()
	firstDir := PluginDir(userDir, "b")
	secondDir := PluginDir(userDir, "a")
	if err := os.MkdirAll(firstDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(secondDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := SaveManifest(firstDir, Manifest{Name: "b", Version: "1.0.0"}); err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}
	if err := SaveManifest(secondDir, Manifest{Name: "a", Version: "1.0.0"}); err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}
	plugins, err := ListInstalled(userDir)
	if err != nil {
		t.Fatalf("ListInstalled: %v", err)
	}
	if len(plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(plugins))
	}
	if plugins[0].Name != "a" || plugins[1].Name != "b" {
		t.Fatalf("unexpected order: %+v", plugins)
	}
}

func TestRemove(t *testing.T) {
	t.Parallel()
	userDir := t.TempDir()
	pluginDir := PluginDir(userDir, "test")
	if err := os.MkdirAll(pluginDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := SaveManifest(pluginDir, Manifest{Name: "test", Version: "1.0.0"}); err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}
	if err := Remove(userDir, "test"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(filepath.Join(pluginDir, "manifest.json")); !os.IsNotExist(err) {
		t.Fatalf("expected plugin dir to be removed")
	}
}
