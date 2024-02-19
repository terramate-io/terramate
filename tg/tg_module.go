// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tg

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"

	"github.com/gruntwork-io/go-commons/env"
	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/configstack"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/util"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/project"
	"github.com/zclconf/go-cty/cty/function"
)

type (
	// Module is a Terragrunt module.
	Module struct {
		Path       project.Path  `json:"path"`
		ConfigFile project.Path  `json:"config"`
		DependsOn  project.Paths `json:"depends_on"`
	}

	// Modules is a list of Module.
	Modules []*Module
)

// ScanModules scans dir looking for Terragrunt modules. It returns a list of
// modules with its "trigger paths" computed. The TriggerPaths are paths that,
// when changed, must mark the module as changed.
func ScanModules(rootdir string, dir project.Path) (Modules, error) {
	absDir := project.AbsPath(rootdir, dir.String())
	opts := newTerragruntOptions(absDir)

	tgConfigFiles, err := config.FindConfigFilesInPath(absDir, opts)
	if err != nil {
		return nil, errors.E(err, "scanning Terragrunt modules")
	}

	logger := log.With().
		Str("action", "tg.ScanModules").
		Logger()

	var modules Modules

	sort.Strings(tgConfigFiles)
	slices.Compact(tgConfigFiles)

	for _, cfgfile := range tgConfigFiles {
		logger := logger.With().Str("cfg-file", cfgfile).Logger()

		logger.Trace().Msg("found configuration")

		cfgOpts := opts.Clone(cfgfile)

		pctx := config.NewParsingContext(context.Background(), cfgOpts).WithDecodeList(
			// needed for tracking:
			//   - terraform.extra_arguments
			//   - terraform.required_vars_file
			//   - terraform.optional_var_files
			//   - etc
			config.TerraformBlock,

			// Needed for detecting modules.
			config.TerraformSource,

			// Need for parsing out the dependencies
			config.DependencyBlock,
		)

		mod := &Module{
			Path:       project.PrjAbsPath(rootdir, cfgOpts.WorkingDir),
			ConfigFile: project.PrjAbsPath(rootdir, cfgfile),
		}

		pctx.PredefinedFunctions = make(map[string]function.Function)
		pctx.PredefinedFunctions[config.FuncNameFindInParentFolders] = findInParentFoldersFunc(pctx, rootdir, mod)
		pctx.PredefinedFunctions[config.FuncNameReadTerragruntConfig] = readTerragruntConfigFunc(pctx, rootdir, mod)
		pctx.PredefinedFunctions[config.FuncNameReadTfvarsFile] = wrapStringSliceToStringAsFuncImpl(pctx, rootdir, mod, readTFVarsFile)

		tgConfig, err := config.PartialParseConfigFile(
			pctx,
			cfgfile,
			nil,
		)

		if err != nil {
			logger.Trace().Err(err).Msgf("ignoring")
			continue
		}

		if tgConfig.Terraform == nil || tgConfig.Terraform.Source == nil {
			// not a runnable module
			continue
		}

		logger.Trace().Msgf("found terraform.source = %q", *tgConfig.Terraform.Source)

		stack, err := configstack.FindStackInSubfolders(cfgOpts, nil)
		if err != nil {
			return nil, errors.E(err, "parsing module at %s", cfgOpts.WorkingDir)
		}

		if len(stack.Modules) == 0 {
			continue
		}

		// first module is the module at cfgfile directory.
		tgMod := stack.Modules[0]
		dependsOn := map[project.Path]struct{}{}
		for _, path := range mod.DependsOn {
			dependsOn[path] = struct{}{}
		}
		// other modules are dependencies.
		for _, dep := range stack.Modules[1:] {
			ppath := project.PrjAbsPath(rootdir, dep.Path)
			if ppath != mod.Path {
				dependsOn[ppath] = struct{}{}
			}
		}
		// TODO(i4k): improve this.
		mod.DependsOn = nil
		for _, include := range tgMod.Config.ProcessedIncludes {
			logger.Trace().Str("include-file", include.Path).Msg("found included file")
			includedFile := include.Path
			if !filepath.IsAbs(includedFile) {
				includedFile = filepath.Join(tgMod.Path, includedFile)
			}
			dependsOn[project.PrjAbsPath(rootdir, includedFile)] = struct{}{}
		}

		for _, dep := range tgMod.Config.TerragruntDependencies {
			logger.Trace().Str("dep-path", dep.ConfigPath).Msg("found dependency")
			depPath := dep.ConfigPath
			if !filepath.IsAbs(depPath) {
				depPath = filepath.Join(tgMod.Path, depPath)
			}
			dependsOn[project.PrjAbsPath(rootdir, depPath)] = struct{}{}
		}

		for p := range dependsOn {
			mod.DependsOn = append(mod.DependsOn, p)
		}
		sort.Slice(mod.DependsOn, func(i, j int) bool {
			return mod.DependsOn[i].String() < mod.DependsOn[j].String()
		})
		modules = append(modules, mod)
	}

	sort.Slice(modules, func(i, j int) bool {
		return modules[i].Path.String() < modules[j].Path.String()
	})
	return modules, nil
}

func newTerragruntOptions(dir string) *options.TerragruntOptions {
	opts := options.NewTerragruntOptions()
	opts.WorkingDir = dir
	opts.Writer = io.Discard
	opts.ErrWriter = io.Discard
	opts.IgnoreExternalDependencies = true
	opts.RunAllAutoApprove = false
	opts.AutoInit = false

	// very important, otherwise the functions could block with user prompts.
	opts.NonInteractive = true

	opts.Env = env.Parse(os.Environ())

	if opts.DisableLogColors {
		util.DisableLogColors()
	}

	opts.DownloadDir = util.JoinPath(opts.WorkingDir, util.TerragruntCacheDir)
	opts.TerragruntConfigPath = config.GetDefaultConfigPath(opts.WorkingDir)

	opts.OriginalTerragruntConfigPath = opts.TerragruntConfigPath
	opts.OriginalTerraformCommand = opts.TerraformCommand
	opts.OriginalIAMRoleOptions = opts.IAMRoleOptions

	return opts
}
