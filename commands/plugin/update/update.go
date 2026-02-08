// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package update provides the plugin update command.
package update

import (
	"context"
	"fmt"

	"github.com/terramate-io/terramate/commands"
	"github.com/terramate-io/terramate/plugin"
)

// Spec represents the plugin update command specification.
type Spec struct {
	PluginName string
}

// Name returns the name of the command.
func (s *Spec) Name() string { return "plugin update" }

// Requirements returns the requirements of the command.
func (s *Spec) Requirements(context.Context, commands.CLI) any { return nil }

// Exec executes the plugin update command.
func (s *Spec) Exec(ctx context.Context, cli commands.CLI) error {
	userDir := cli.Config().UserTerramateDir
	if userDir == "" {
		return fmt.Errorf("user terramate dir not configured")
	}
	if s.PluginName != "" {
		manifest, err := plugin.Install(ctx, s.PluginName, plugin.InstallOptions{
			UserTerramateDir: userDir,
		})
		if err != nil {
			return err
		}
		cli.Printers().Stdout.Println(fmt.Sprintf("Updated %s to %s", manifest.Name, manifest.Version))
		return nil
	}
	plugins, err := plugin.ListInstalled(userDir)
	if err != nil {
		return err
	}
	if len(plugins) == 0 {
		cli.Printers().Stdout.Println("No plugins installed.")
		return nil
	}
	for _, p := range plugins {
		manifest, err := plugin.Install(ctx, p.Name, plugin.InstallOptions{
			UserTerramateDir: userDir,
			RegistryURL:      p.Registry,
		})
		if err != nil {
			return err
		}
		cli.Printers().Stdout.Println(fmt.Sprintf("Updated %s to %s", manifest.Name, manifest.Version))
	}
	return nil
}
