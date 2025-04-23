// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tg

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/util"
	"github.com/terramate-io/terramate"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/fs"
)

func findConfigFilesInPath(rootPath string, terragruntOptions *options.TerragruntOptions) ([]string, error) {
	ok, err := isTerragruntModuleDir(rootPath, terragruntOptions)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}

	fileList, err := fs.ListTerramateFiles(rootPath)
	if err != nil {
		return nil, errors.E(err, "listing terragrunt config files in %s", rootPath)
	}

	configFiles := []string{}
	if slices.Contains(fileList.Skipped, terramate.SkipFilename) {
		return configFiles, nil
	}

	if fileList.TgRootFile != "" {
		configFiles = append(configFiles, util.JoinPath(rootPath, fileList.TgRootFile))
	}

	for _, dir := range fileList.Dirs {
		configFilesInDir, err := findConfigFilesInPath(filepath.Join(rootPath, dir), terragruntOptions)
		if err != nil {
			return nil, err
		}
		configFiles = append(configFiles, configFilesInDir...)
	}
	return configFiles, err
}

// isTerragruntModuleDir returns true if the given path contains a Terragrunt module and false otherwise.
// The path can not contain a cache, data, or download dir.
//
// Note(i4k): this function is a copy of the one in terragrunt v0.55.21. It actually does not check if the path
// contains a terragrunt module but only **if it is a valid path** to a terragrunt module.
func isTerragruntModuleDir(path string, terragruntOptions *options.TerragruntOptions) (bool, error) {
	// Skip the Terragrunt cache dir
	if util.ContainsPath(path, util.TerragruntCacheDir) {
		return false, nil
	}

	// Skip the Terraform data dir
	dataDir := terragruntOptions.TerraformDataDir()
	if filepath.IsAbs(dataDir) {
		if util.HasPathPrefix(path, dataDir) {
			return false, nil
		}
	} else {
		if util.ContainsPath(path, dataDir) {
			return false, nil
		}
	}

	canonicalPath, err := util.CanonicalPath(path, "")
	if err != nil {
		return false, err
	}

	canonicalDownloadPath, err := util.CanonicalPath(terragruntOptions.DownloadDir, "")
	if err != nil {
		return false, err
	}

	// Skip any custom download dir specified by the user
	if strings.Contains(canonicalPath, canonicalDownloadPath) {
		return false, nil
	}

	return true, nil
}
