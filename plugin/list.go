// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"os"
	"path/filepath"
	"sort"
)

// ListInstalled returns all installed plugin manifests.
func ListInstalled(userTerramateDir string) ([]Manifest, error) {
	pluginsDir := PluginsDir(userTerramateDir)
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var manifests []Manifest
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pluginDir := filepath.Join(pluginsDir, entry.Name())
		manifest, err := LoadManifest(pluginDir)
		if err != nil {
			continue
		}
		manifests = append(manifests, manifest)
	}
	sort.Slice(manifests, func(i, j int) bool {
		return manifests[i].Name < manifests[j].Name
	})
	return manifests, nil
}

// Remove deletes an installed plugin.
func Remove(userTerramateDir, name string) error {
	return os.RemoveAll(PluginDir(userTerramateDir, name))
}
