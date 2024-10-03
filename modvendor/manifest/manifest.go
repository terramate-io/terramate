// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package manifest implements vendor manifest parsing.
package manifest

import (
	stdos "os"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/os"
)

// LoadFileMatcher will load a gitignore.Matcher
func LoadFileMatcher(rootdir os.Path) (gitignore.Matcher, error) {
	dotTerramate := rootdir.Join(".terramate")
	dotTerramateInfo, err := stdos.Stat(dotTerramate.String())

	if err == nil && dotTerramateInfo.IsDir() {
		cfg, err := hcl.ParseDir(rootdir, dotTerramate)
		if err != nil {
			return nil, errors.E(err, "parsing manifest on .terramate")
		}
		if hasVendorManifest(cfg) {
			return newMatcher(cfg), nil
		}
	}

	cfg, err := hcl.ParseDir(rootdir, rootdir)
	if err != nil {
		return nil, errors.E(err, "parsing manifest on project root")
	}

	if hasVendorManifest(cfg) {
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
