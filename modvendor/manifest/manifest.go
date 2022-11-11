// Copyright 2022 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package manifest implements vendor manifest parsing.
package manifest

import (
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/rs/zerolog/log"
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
