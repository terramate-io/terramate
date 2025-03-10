// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package engine

import (
	"os"
	"path"
	"path/filepath"

	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/project"
)

const defaultVendorDir = "/modules"

func (e *Engine) VendorDir() (project.Path, error) {
	checkVendorDir := func(dir string) (project.Path, error) {
		if !path.IsAbs(dir) {
			return project.InvalidPath(), errors.E("vendorDir %s defined is not an absolute path", dir)
		}
		return project.NewPath(dir), nil
	}

	dotTerramate := filepath.Join(e.rootdir(), ".terramate")
	dotTerramateInfo, err := os.Stat(dotTerramate)

	if err == nil && dotTerramateInfo.IsDir() {

		cfg, err := hcl.ParseDir(e.rootdir(), filepath.Join(e.rootdir(), ".terramate"))
		if err != nil {
			return project.InvalidPath(), errors.E(err, "parsing vendor dir configuration on .terramate")
		}

		if hasVendorDirConfig(cfg) {
			return checkVendorDir(cfg.Vendor.Dir)
		}
	}

	hclcfg := e.RootNode()
	if hasVendorDirConfig(hclcfg) {
		return checkVendorDir(hclcfg.Vendor.Dir)
	}

	return project.NewPath(defaultVendorDir), nil
}

func hasVendorDirConfig(cfg hcl.Config) bool {
	return cfg.Vendor != nil && cfg.Vendor.Dir != ""
}
