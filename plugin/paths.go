// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package plugin

import "path/filepath"

const pluginsDirName = "plugins"

// PluginsDir returns the base directory for plugins under the user terramate dir.
func PluginsDir(userTerramateDir string) string {
	return filepath.Join(userTerramateDir, pluginsDirName)
}

// PluginDir returns the directory for a single plugin.
//
//revive:disable-next-line:exported
func PluginDir(userTerramateDir, name string) string {
	return filepath.Join(PluginsDir(userTerramateDir), name)
}
