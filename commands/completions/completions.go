// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package completions provides the install-completions command.
package completions

import (
	"context"

	"github.com/alecthomas/kong"
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

// Exec executes the install-completions command.
func (s *Spec) Exec(_ context.Context) error {
	err := s.Installer.Run(s.KongCtx)
	if err != nil {
		return errors.E(err, "installing shell completions")
	}
	return nil
}
