// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestParseNameVersion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input       string
		wantName    string
		wantVersion string
	}{
		{"catalyst", "catalyst", ""},
		{"catalyst@1.0.0", "catalyst", "1.0.0"},
		{"", "", ""},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			name, version := ParseNameVersion(tt.input)
			if name != tt.wantName || version != tt.wantVersion {
				t.Fatalf("got %q %q", name, version)
			}
		})
	}
}

func TestInstallFromLocal(t *testing.T) {
	t.Parallel()
	userDir := t.TempDir()
	sourceDir := t.TempDir()
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	cliPath := filepath.Join(sourceDir, "terramate"+suffix)
	buildTestGRPCPlugin(t, cliPath)

	manifest, err := InstallFromLocal(context.Background(), "test", InstallOptions{
		UserTerramateDir: userDir,
		Source:           sourceDir,
	})
	if err != nil {
		t.Fatalf("InstallFromLocal: %v", err)
	}
	if manifest.Name != "test" || manifest.Version != "local" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	pluginDir := PluginDir(userDir, "test")
	loaded, err := LoadManifest(pluginDir)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if _, ok := loaded.Binaries[BinaryCLI]; !ok {
		t.Fatalf("expected cli binary")
	}
}

func TestInstallFromLocalMissingBinaries(t *testing.T) {
	t.Parallel()
	userDir := t.TempDir()
	sourceDir := t.TempDir()
	_, err := InstallFromLocal(context.Background(), "test", InstallOptions{
		UserTerramateDir: userDir,
		Source:           sourceDir,
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func buildTestGRPCPlugin(t *testing.T, outputPath string) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Clean(filepath.Join(wd, ".."))
	cmd := exec.Command("go", "build", "-o", outputPath, "./e2etests/cmd/grpcplugin")
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build grpc plugin: %v (%s)", err, string(out))
	}
}
