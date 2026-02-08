// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package grpc provides gRPC plugin discovery utilities.
package grpc

import (
	"path/filepath"

	"github.com/terramate-io/terramate/plugin"
)

// InstalledPlugin represents a discovered gRPC-capable plugin binary.
type InstalledPlugin struct {
	Manifest   plugin.Manifest
	PluginDir  string
	BinaryPath string
}

// DiscoverInstalled returns gRPC-capable plugins based on their manifests.
func DiscoverInstalled(userTerramateDir string) ([]InstalledPlugin, error) {
	manifests, err := plugin.ListInstalled(userTerramateDir)
	if err != nil {
		return nil, err
	}

	var plugins []InstalledPlugin
	for _, m := range manifests {
		if !hasGRPCProtocol(m) {
			continue
		}

		bin, ok := m.Binaries[plugin.BinaryCLI]
		if !ok || bin.Path == "" {
			continue
		}

		pluginDir := plugin.PluginDir(userTerramateDir, m.Name)
		plugins = append(plugins, InstalledPlugin{
			Manifest:   m,
			PluginDir:  pluginDir,
			BinaryPath: filepath.Join(pluginDir, bin.Path),
		})
	}
	return plugins, nil
}

func hasGRPCProtocol(m plugin.Manifest) bool {
	if m.Protocol == plugin.ProtocolGRPC {
		return true
	}
	if m.Metadata == nil {
		return false
	}
	v, ok := m.Metadata["protocol"]
	if !ok {
		return false
	}
	s, ok := v.(string)
	return ok && s == "grpc"
}
