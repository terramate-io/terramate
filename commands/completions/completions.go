// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package completions

import (
	"context"

	"github.com/alecthomas/kong"
	"github.com/terramate-io/terramate/errors"
	"github.com/willabides/kongplete"
)

type Spec struct {
	Installer kongplete.InstallCompletions
	KongCtx   *kong.Context
}

func (s *Spec) Name() string { return "install-completions" }

func (s *Spec) Exec(_ context.Context) error {
	err := s.Installer.Run(s.KongCtx)
	if err != nil {
		return errors.E(err, "installing shell completions")
	}
	return nil
}
