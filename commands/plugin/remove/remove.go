// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package remove provides the plugin remove command.
package remove

import (
	"context"
	"fmt"

	"github.com/terramate-io/terramate/commands"
	"github.com/terramate-io/terramate/plugin"
)

// Spec represents the plugin remove command specification.
type Spec struct {
	PluginName string
}

// Name returns the name of the command.
func (s *Spec) Name() string { return "plugin remove" }

// Requirements returns the requirements of the command.
func (s *Spec) Requirements(context.Context, commands.CLI) any { return nil }

// Exec executes the plugin remove command.
func (s *Spec) Exec(_ context.Context, cli commands.CLI) error {
	if s.PluginName == "" {
		return fmt.Errorf("plugin name is required")
	}
	userDir := cli.Config().UserTerramateDir
	if userDir == "" {
		return fmt.Errorf("user terramate dir not configured")
	}
	if err := plugin.Remove(userDir, s.PluginName); err != nil {
		return err
	}
	cli.Printers().Stdout.Println(fmt.Sprintf("Removed %s", s.PluginName))
	return nil
}
