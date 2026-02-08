// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package list provides the plugin list command.
package list

import (
	"context"
	"fmt"

	"github.com/terramate-io/terramate/commands"
	"github.com/terramate-io/terramate/plugin"
)

// Spec represents the plugin list command specification.
type Spec struct{}

// Name returns the name of the command.
func (s *Spec) Name() string { return "plugin list" }

// Requirements returns the requirements of the command.
func (s *Spec) Requirements(context.Context, commands.CLI) any { return nil }

// Exec executes the plugin list command.
func (s *Spec) Exec(_ context.Context, cli commands.CLI) error {
	userDir := cli.Config().UserTerramateDir
	if userDir == "" {
		return fmt.Errorf("user terramate dir not configured")
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
		cli.Printers().Stdout.Println(fmt.Sprintf("%s %s (%s)", p.Name, p.Version, p.Type))
	}
	return nil
}
