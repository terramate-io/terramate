// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package completions provides the install-completions command.
package completions

import (
	"context"

	"github.com/alecthomas/kong"
	"github.com/terramate-io/terramate/commands"
	"github.com/terramate-io/terramate/errors"
	"github.com/willabides/kongplete"
)

// Spec is the command specification for the install-completions command.
type Spec struct {
	Installer kongplete.InstallCompletions
	KongCtx   *kong.Context
}

// Name returns the name of the command.
func (s *Spec) Name() string { return "install-completions" }

// Requirements returns the requirements of the command.
func (s *Spec) Requirements(context.Context, commands.CLI) any { return nil }

// Exec executes the install-completions command.
func (s *Spec) Exec(_ context.Context, _ commands.CLI) error {
	err := s.Installer.Run(s.KongCtx)
	if err != nil {
		return errors.E(err, "installing shell completions")
	}
	return nil
}
