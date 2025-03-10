// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package run

import (
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/generate"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/safeguard"
)

func (s *Spec) checkOutdatedGeneratedCode() error {
	logger := log.With().
		Str("action", "checkOutdatedGeneratedCode()").
		Logger()

	if !checkGenCode(s.Engine, s.Safeguards) {
		return nil
	}

	cfg := s.Engine.Config()
	wdpath := project.PrjAbsPath(cfg.HostDir(), s.WorkingDir)
	targetTree, ok := cfg.Lookup(wdpath)
	if !ok {
		return errors.E("config not found at %s", wdpath)
	}

	vendorDir, err := s.Engine.VendorDir()
	if err != nil {
		return err
	}
	outdatedFiles, err := generate.DetectOutdated(cfg, targetTree, vendorDir)
	if err != nil {
		return errors.E(err, "failed to check outdated code on project")
	}

	for _, outdated := range outdatedFiles {
		logger.Error().
			Str("filename", outdated).
			Msg("outdated code found")
	}

	if len(outdatedFiles) > 0 {
		return errors.E(errors.E("please run: 'terramate generate' to update generated code"),
			errors.E(ErrOutdatedGenCodeDetected).Error(),
		)
	}
	return nil
}

func checkGenCode(engine *engine.Engine, safeguards Safeguards) bool {
	if safeguards.DisableCheckGenerateOutdatedCheck {
		return false
	}

	if safeguards.reEnabled {
		return !safeguards.DisableCheckGenerateOutdatedCheck
	}

	cfg := engine.RootNode()
	if cfg.Terramate == nil || cfg.Terramate.Config == nil {
		return true
	}
	return !cfg.Terramate.Config.HasSafeguardDisabled(safeguard.Outdated)

}
