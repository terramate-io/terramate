// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tg

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gruntwork-io/go-commons/env"
	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/configstack"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/util"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/printer"
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
		pctx.PredefinedFunctions[config.FuncNameFindInParentFolders] = tgFindInParentFoldersFuncImpl(pctx, rootdir, mod)
		pctx.PredefinedFunctions[config.FuncNameReadTerragruntConfig] = tgReadTerragruntConfigFuncImpl(pctx, rootdir, mod)
		pctx.PredefinedFunctions[config.FuncNameReadTfvarsFile] = wrapStringSliceToStringAsFuncImpl(pctx, rootdir, mod, tgReadTFVarsFileFuncImpl)

		// override Terraform function
		pctx.PredefinedFunctions["file"] = tgFileFuncImpl(pctx, rootdir, mod)

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

		var tgMod *configstack.TerraformModule
		for _, m := range stack.Modules {
			if m.Path == cfgOpts.WorkingDir {
				tgMod = m
				break
			}
		}

		// sanity check
		if tgMod == nil {
			panic(errors.E(errors.ErrInternal, "%s found but module not found in subfolders. Please report this bug.", cfgfile))
		}

		mod.Source = *tgConfig.Terraform.Source

		dependsOn := map[project.Path]struct{}{}

		_, err = os.Lstat(mod.Source)
		// if the source is a directory, we assume it is a local module.
		if err == nil && filepath.IsAbs(mod.Source) {
			src, err := filepath.EvalSymlinks(mod.Source)
			if err != nil {
				return nil, errors.E(err, "evaluating symlinks in %q", mod.Source)
			}
			// we normalize local paths as relative to the module.
			// so this is compatible with Terraform module sources.
			rel, err := filepath.Rel(cfgOpts.WorkingDir, src)
			if err != nil {
				return nil, errors.E(err, "normalizing local path %q", mod.Source)
			}
			mod.Source = rel
		}

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
				if dep.Enabled != nil && !*dep.Enabled {
					continue
				}
				depAbsPath := dep.ConfigPath
				if !filepath.IsAbs(depAbsPath) {
					depAbsPath = filepath.Join(tgMod.Path, depAbsPath)
				}

				logger.Trace().
					Str("mod-path", tgMod.Path).
					Str("dep-path", dep.ConfigPath).
					Str("dep-abs-path", depAbsPath).
					Msg("found dependency (in dependency.config_path)")

				if depAbsPath != rootdir && !strings.HasPrefix(depAbsPath, rootdir+string(filepath.Separator)) {
					warnDependencyOutsideProject(mod, depAbsPath, "dependency.config_path")

					continue
				}

				depProjectPath := project.PrjAbsPath(rootdir, depAbsPath)

				dependsOn[depProjectPath] = struct{}{}
			}
		}

		for p := range dependsOn {
			dependsAbsPath := project.AbsPath(rootdir, p.String())
			fileProcessed[dependsAbsPath] = struct{}{}
			mod.DependsOn = append(mod.DependsOn, p)
		}

		if tgConfig.Dependencies != nil {
			for _, depPath := range tgConfig.Dependencies.Paths {
				depAbsPath := depPath
				if !filepath.IsAbs(depPath) {
					depAbsPath = filepath.Join(tgMod.Path, filepath.FromSlash(depPath))
				}

				logger.Trace().
					Str("mod-path", mod.Path.String()).
					Str("dep-path", depPath).
					Str("dep-abs-path", depAbsPath).
					Msg("found dependency (in dependencies.paths)")

				if depPath != rootdir && !strings.HasPrefix(depAbsPath, rootdir+string(filepath.Separator)) {
					warnDependencyOutsideProject(mod, depAbsPath, "dependencies.paths")

					continue
				}

				depProjectPath := project.PrjAbsPath(rootdir, depAbsPath)

				mod.After = append(mod.After, depProjectPath)
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

func warnDependencyOutsideProject(mod *Module, dep string, field string) {
	printer.Stderr.WarnWithDetails(fmt.Sprintf("Dependency outside of Terramate project detected in `%s` configuration. Ignoring.", field),
		errors.E("The Terragrunt module %s depends on the module at %s, which is located outside of the your current"+
			" Terramate project. To resolve this ensure the dependent module is part of your Terramate project.",
			mod.Path, dep),
	)
}
