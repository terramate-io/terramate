// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tg

import (
	"context"
	"io"
	"os"
	"path"
	"path/filepath"
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

// ErrParsing indicates there is an error parsing a Terragrunt file.
const ErrParsing errors.Kind = "parsing Terragrunt file"

type (
	// Module is a Terragrunt module.
	Module struct {
		Path       project.Path  `json:"path"`
		Source     string        `json:"source"`
		ConfigFile project.Path  `json:"config"`
		After      project.Paths `json:"after,omitempty"`

		// DependsOn are paths that, when changed, must mark the module as changed.
		DependsOn project.Paths `json:"depends_on,omitempty"`
	}

	// Modules is a list of Module.
	Modules []*Module
)

// ScanModules scans dir looking for Terragrunt modules. It returns a list of
// modules with its "DependsOn paths" computed.
func ScanModules(rootdir string, dir project.Path, trackDependencies bool) (Modules, error) {
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

	fileErrs := map[string]*errors.List{}
	fileProcessed := map[string]struct{}{}
	for _, cfgfile := range tgConfigFiles {
		fileErrs[cfgfile] = errors.L()
		logger := logger.With().Str("cfg-file", cfgfile).Logger()

		logger.Trace().Msg("found configuration")

		cfgOpts := opts.Clone(cfgfile)

		decodeOptions := []config.PartialDecodeSectionType{
			// needed for tracking:
			//   - terraform.extra_arguments
			//   - terraform.required_vars_file
			//   - terraform.optional_var_files
			//   - etc
			config.TerraformBlock,

			// Needed for detecting modules.
			config.TerraformSource,

			// for ordering
			config.DependenciesBlock,
		}

		if trackDependencies {
			// Need for parsing out the dependencies
			decodeOptions = append(decodeOptions, config.DependencyBlock)
		}

		pctx := config.NewParsingContext(context.Background(), cfgOpts).WithDecodeList(decodeOptions...)
		mod := &Module{
			Path:       project.PrjAbsPath(rootdir, cfgOpts.WorkingDir),
			ConfigFile: project.PrjAbsPath(rootdir, cfgfile),
		}

		// Override the predefined functions to intercept the function calls that process paths.
		pctx.PredefinedFunctions = make(map[string]function.Function)
		pctx.PredefinedFunctions[config.FuncNameFindInParentFolders] = findInParentFoldersFunc(pctx, rootdir, mod)
		pctx.PredefinedFunctions[config.FuncNameReadTerragruntConfig] = readTerragruntConfigFunc(pctx, rootdir, mod)
		pctx.PredefinedFunctions[config.FuncNameReadTfvarsFile] = wrapStringSliceToStringAsFuncImpl(pctx, rootdir, mod, readTFVarsFile)

		// Here we parse the Terragrunt file which calls into our overrided functions.
		// After this returns, the module's DependsOn will be populated.
		// Note(i4k): Never use `config.ParseConfigFile` because it invokes Terraform behind the scenes.
		tgConfig, err := config.PartialParseConfigFile(
			pctx,
			cfgfile,
			nil,
		)

		if err != nil {
			fileErrs[cfgfile].AppendWrap(ErrParsing, err)
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
		mod.Source = *tgConfig.Terraform.Source
		dependsOn := map[project.Path]struct{}{}
		for _, path := range mod.DependsOn {
			dependsOn[path] = struct{}{}
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

		if trackDependencies {
			// "dependency" block is TerragruntDependencies
			// they get automatically added into tgConfig.Dependencies.Paths
			for _, dep := range tgConfig.TerragruntDependencies {
				logger.Trace().Str("dep-path", dep.ConfigPath).Msg("found dependency")
				if dep.Enabled != nil && !*dep.Enabled {
					continue
				}
				depPath := dep.ConfigPath
				if !filepath.IsAbs(depPath) {
					depPath = filepath.Join(tgMod.Path, depPath)
				}
				dependsOn[project.PrjAbsPath(rootdir, depPath)] = struct{}{}
			}
		}

		for p := range dependsOn {
			dependsAbsPath := project.AbsPath(rootdir, p.String())
			fileProcessed[dependsAbsPath] = struct{}{}
			mod.DependsOn = append(mod.DependsOn, p)
		}

		if tgConfig.Dependencies != nil {
			for _, dep := range tgConfig.Dependencies.Paths {
				var p project.Path
				if !path.IsAbs(dep) {
					p = mod.Path.Join(dep)
				} else {
					logger.Debug().Str("dep-path", dep).Msg("ignore absolute path")
					continue
				}
				mod.After = append(mod.After, p)
			}
			mod.After.Sort()
		}
		sort.Slice(mod.DependsOn, func(i, j int) bool {
			return mod.DependsOn[i].String() < mod.DependsOn[j].String()
		})
		modules = append(modules, mod)
	}

	errs := errors.L()
	for path, ferr := range fileErrs {
		if _, ok := fileProcessed[path]; !ok {
			errs.Append(ferr.AsError())
		}
	}
	if err := errs.AsError(); err != nil {
		return nil, err
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
