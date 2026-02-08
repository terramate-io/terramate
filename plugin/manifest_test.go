// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadManifest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	want := Manifest{
		Name:    "test",
		Version: "1.0.0",
		Type:    TypeGRPC,
		Binaries: map[BinaryKind]Binary{
			BinaryCLI: {Path: "terramate"},
		},
	}
	if err := SaveManifest(dir, want); err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}
	got, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if got.Name != want.Name || got.Version != want.Version || got.Type != want.Type {
		t.Fatalf("unexpected manifest: %+v", got)
	}
	if got.Binaries[BinaryCLI].Path != "terramate" {
		t.Fatalf("unexpected binaries: %+v", got.Binaries)
	}
}

func TestLoadManifestNotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := LoadManifest(dir)
	if err == nil {
		t.Fatal("expected error for missing manifest")
	}
	if !os.IsNotExist(err) {
		t.Fatalf("expected not-exist error, got %v", err)
	}
}

func TestManifestPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	got := ManifestPath(dir)
	want := filepath.Join(dir, "manifest.json")
	if got != want {
		t.Fatalf("unexpected path: %s", got)
	}
}
