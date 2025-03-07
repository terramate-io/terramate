package requiredversion

import (
	"context"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/versions"
)

type Spec struct {
	Version string
	Root    *config.Root
}

func (s *Spec) Exec(ctx context.Context) error {
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
