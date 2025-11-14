// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package requiredversion provides the required-version command.
package requiredversion

import (
	"context"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/versions"
)

// Note(snk): This is not a real command?

// Spec is the command specification for the required-version command.
type Spec struct {
	Version string
	Root    *config.Root
}

// Name returns the name of the command.
func (s *Spec) Name() string { return "required-version" }

// Exec executes the required-version command.
func (s *Spec) Exec(_ context.Context) error {
	logger := log.With().
		Str("action", "commands.requiredversion.Exec()").
		Str("version", s.Version).
		Logger()

	rootcfg := s.Root.Tree().Node
	if rootcfg.Terramate == nil {
		logger.Debug().Msg("project root has no config, skipping version check")
		return nil
	}
	if rootcfg.Terramate.RequiredVersion == "" {
		logger.Debug().Msg("project root config has no required_version, skipping version check")
		return nil
	}
	err := versions.Check(
		s.Version,
		rootcfg.Terramate.RequiredVersion,
		rootcfg.Terramate.RequiredVersionAllowPreReleases,
	)
	if err != nil {
		logger.Debug().
			Str("required_version", rootcfg.Terramate.RequiredVersion).
			Bool("required_version_allow_prereleases", rootcfg.Terramate.RequiredVersionAllowPreReleases).
			Msg("version check failed")

		return errors.E(err, "version check failed")
	}
	return nil
}
