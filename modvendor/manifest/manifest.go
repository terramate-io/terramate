// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package manifest implements vendor manifest parsing.
package manifest

import (
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
)

// LoadFileMatcher will load a gitignore.Matcher
func LoadFileMatcher(rootdir string) (gitignore.Matcher, error) {
	logger := log.With().
		Str("action", "modvendor.loadFileMatcher").
		Str("rootdir", rootdir).
		Logger()

	logger.Trace().Msg("checking for manifest on .terramate")

	dotTerramate := filepath.Join(rootdir, ".terramate")
	dotTerramateInfo, err := os.Stat(dotTerramate)

	if err == nil && dotTerramateInfo.IsDir() {
		cfg, err := hcl.ParseDir(rootdir, dotTerramate)
		if err != nil {
			return nil, errors.E(err, "parsing manifest on .terramate")
		}
		if hasVendorManifest(cfg) {
			logger.Trace().Msg("found manifest on .terramate")
			return newMatcher(cfg), nil
		}
	}

	logger.Trace().Msg("checking for manifest on root")

	cfg, err := hcl.ParseDir(rootdir, rootdir)
	if err != nil {
		return nil, errors.E(err, "parsing manifest on project root")
	}

	if hasVendorManifest(cfg) {
		logger.Trace().Msg("found manifest on root")
		return newMatcher(cfg), nil
	}

	return defaultMatcher(), nil
}

func newMatcher(cfg hcl.Config) gitignore.Matcher {
	files := cfg.Vendor.Manifest.Default.Files
	patterns := make([]gitignore.Pattern, len(files))
	for i, rawPattern := range files {
		patterns[i] = gitignore.ParsePattern(rawPattern, nil)
	}
	return gitignore.NewMatcher(patterns)
}

func defaultMatcher() gitignore.Matcher {
	return gitignore.NewMatcher([]gitignore.Pattern{
		gitignore.ParsePattern("**", nil),
		gitignore.ParsePattern("!/.terramate", nil),
	})
}

func hasVendorManifest(cfg hcl.Config) bool {
	return cfg.Vendor != nil &&
		cfg.Vendor.Manifest != nil &&
		cfg.Vendor.Manifest.Default != nil &&
		len(cfg.Vendor.Manifest.Default.Files) > 0
}
