// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const manifestFilename = "manifest.json"

// Type represents the plugin type.
type Type string

const (
	// TypeGRPC indicates a gRPC plugin.
	TypeGRPC Type = "grpc"
)

// Protocol represents how a plugin is loaded.
type Protocol string

const (
	// ProtocolGRPC indicates gRPC plugin protocol.
	ProtocolGRPC Protocol = "grpc"
)

// BinaryKind identifies a binary entry inside a manifest.
type BinaryKind string

const (
	// BinaryCLI identifies the plugin CLI binary.
	BinaryCLI BinaryKind = "cli"
	// BinaryLS identifies the plugin language server binary.
	BinaryLS BinaryKind = "ls"
)

// Binary describes an installed plugin binary.
type Binary struct {
	Path      string `json:"path"`
	Checksum  string `json:"checksum,omitempty"`
	Signature string `json:"signature,omitempty"`
	PublicKey string `json:"public_key,omitempty"`
}

// Manifest describes an installed plugin.
type Manifest struct {
	Name           string                 `json:"name"`
	Version        string                 `json:"version"`
	Type           Type                   `json:"type"`
	Protocol       Protocol               `json:"protocol,omitempty"`
	CompatibleWith string                 `json:"compatible_with,omitempty"`
	Binaries       map[BinaryKind]Binary  `json:"binaries"`
	Registry       string                 `json:"registry,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// ManifestPath returns the manifest path in the plugin directory.
func ManifestPath(pluginDir string) string {
	return filepath.Join(pluginDir, manifestFilename)
}

// LoadManifest loads the manifest from pluginDir.
func LoadManifest(pluginDir string) (Manifest, error) {
	var m Manifest
	content, err := os.ReadFile(ManifestPath(pluginDir))
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(content, &m); err != nil {
		return m, err
	}
	return m, nil
}

// SaveManifest writes the manifest to pluginDir.
func SaveManifest(pluginDir string, m Manifest) error {
	content, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	return os.WriteFile(ManifestPath(pluginDir), content, 0o600)
}
