// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package add provides the plugin add command.
package add

import (
	"context"
	"fmt"

	"github.com/terramate-io/terramate/commands"
	"github.com/terramate-io/terramate/plugin"
)

// Spec represents the plugin add command specification.
type Spec struct {
	PluginName string
	Source     string
}

// Name returns the name of the command.
func (s *Spec) Name() string { return "plugin add" }

// Requirements returns the requirements of the command.
func (s *Spec) Requirements(context.Context, commands.CLI) any { return nil }

// Exec executes the plugin add command.
func (s *Spec) Exec(ctx context.Context, cli commands.CLI) error {
	name, version := plugin.ParseNameVersion(s.PluginName)
	if name == "" {
		return fmt.Errorf("plugin name is required")
	}
	userDir := cli.Config().UserTerramateDir
	if userDir == "" {
		return fmt.Errorf("user terramate dir not configured")
	}
	manifest, err := plugin.Install(ctx, name, plugin.InstallOptions{
		UserTerramateDir: userDir,
		Version:          version,
		Source:           s.Source,
	})
	if err != nil {
		return err
	}
	cli.Printers().Stdout.Println(fmt.Sprintf("Installed %s %s", manifest.Name, manifest.Version))
	return nil
}
