// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package plugin

import "github.com/terramate-io/terramate/ui/tui/cliconfig"

// ResolveUserTerramateDir returns the configured or default terramate user dir.
func ResolveUserTerramateDir() (string, error) {
	cfg, err := cliconfig.Load()
	if err != nil {
		return "", err
	}
	if cfg.UserTerramateDir != "" {
		return cfg.UserTerramateDir, nil
	}
	return DefaultUserTerramateDir()
}
