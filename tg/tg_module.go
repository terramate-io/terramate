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
	"sync"

	"github.com/gruntwork-io/go-commons/env"
	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/configstack"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/util"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/project"
	"github.com/zclconf/go-cty/cty"
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

		FilesProcessed map[string]struct{}
		parseCache     *ParseCache
	}

	// Modules is a list of Module.
	Modules []*Module

	// ParseCache caches parsed Terragrunt config files to avoid re-parsing the same file multiple times.
	// This is exported so it can be shared across parallel workers in config loading.
	//
	// LIMITATION: The cache assumes deterministic evaluation within a single run. If Terragrunt
	// configs use conditional logic based on per-module context to determine which files to include
	// (e.g., ternary operators in read_terragrunt_config() paths), the cached dependencies may be
	// incomplete for some modules. This is rare in practice but can cause false negatives in change
	// detection (missing dependencies). To disable caching, use --disable-tg-cache flag or set
	// TM_DISABLE_TG_CACHE=1 environment variable.
	ParseCache struct {
		mu     sync.RWMutex
		parsed map[string]*tgCacheEntry
	}

	// tgCacheEntry stores both the parsed value and dependencies discovered during parsing.
	tgCacheEntry struct {
		value        cty.Value
		dependencies project.Paths
	}
)

var (
	// defaultParseCache is a package-level cache used by LoadModule to enable
	// caching across multiple LoadModule calls without explicit cache management.
	defaultParseCache     *ParseCache
	defaultParseCacheOnce sync.Once
)

func init() {
	// Logger colors are now handled by the logger implementation
}

func isCachingDisabled() bool {
	return os.Getenv("TM_DISABLE_TG_CACHE") != ""
}

func getDefaultParseCache() *ParseCache {
	if isCachingDisabled() {
		return nil
	}
	defaultParseCacheOnce.Do(func() {
		defaultParseCache = NewParseCache()
	})
	return defaultParseCache
}

// NewParseCache creates a new Terragrunt parse cache.
// This is exported so config loading can create a shared cache across workers.
func NewParseCache() *ParseCache {
	return &ParseCache{
		parsed: make(map[string]*tgCacheEntry),
	}
}

func (c *ParseCache) get(path string) (*tgCacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.parsed[path]
	return entry, ok
}

func (c *ParseCache) set(path string, entry *tgCacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.parsed[path] = entry
}

// LoadModule loads a Terragrunt module from dir and fname.
//
// If fname is not absolute, it is considered to be relative to dir.
// This function uses a package-level shared cache for performance.
// For explicit cache control, use LoadModuleWithCache.
func LoadModule(rootdir string, dir project.Path, fname string, trackDependencies bool) (mod *Module, isRootModule bool, err error) {
	return LoadModuleWithCache(rootdir, dir, fname, trackDependencies, getDefaultParseCache())
}

// LoadModuleWithCache loads a Terragrunt module with a shared parse cache.
// This is exported so config loading can share a cache across parallel workers.
func LoadModuleWithCache(rootdir string, dir project.Path, fname string, trackDependencies bool, cache *ParseCache) (mod *Module, isRootModule bool, err error) {
	logger := log.With().
		Str("action", "tg.LoadModule").
		Str("dir", dir.String()).
		Str("cfgfile", fname).
		Logger()

	absDir := project.AbsPath(rootdir, dir.String())
	cfgfile := filepath.Join(absDir, fname)
	cfgOpts := newTerragruntOptions(absDir, cfgfile)

	// Create a Terragrunt-compatible logger with cleanup function
	tgLogger, cleanup := NewTerragruntLogger(logger)
	defer func() {
		if closeErr := cleanup(); closeErr != nil {
			logger.Warn().Err(closeErr).Msg("failed to close terragrunt logger")
		}
	}()

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

	pctx := config.NewParsingContext(context.Background(), tgLogger, cfgOpts).WithDecodeList(decodeOptions...)
	mod = &Module{
		Path:           project.PrjAbsPath(rootdir, cfgOpts.WorkingDir),
		ConfigFile:     project.PrjAbsPath(rootdir, cfgfile),
		FilesProcessed: map[string]struct{}{},
		parseCache:     cache,
	}

	// Override the predefined functions to intercept the function calls that process paths.
	pctx.PredefinedFunctions = make(map[string]function.Function)
	pctx.PredefinedFunctions[config.FuncNameFindInParentFolders] = tgFindInParentFoldersFuncImpl(pctx, tgLogger, rootdir, mod)
	pctx.PredefinedFunctions[config.FuncNameReadTerragruntConfig] = tgReadTerragruntConfigFuncImpl(pctx, tgLogger, rootdir, mod)
	pctx.PredefinedFunctions[config.FuncNameReadTfvarsFile] = wrapStringSliceToStringAsFuncImpl(pctx, tgLogger, rootdir, mod, tgReadTFVarsFileFuncImpl)

	// override Terraform function
	pctx.PredefinedFunctions["file"] = tgFileFuncImpl(pctx, rootdir, mod)

	// Here we parse the Terragrunt file which calls into our overrided functions.
	// After this returns, the module's DependsOn will be populated.
	// Note(i4k): Never use `config.ParseConfigFile` because it invokes Terraform behind the scenes.
	tgConfig, err := config.PartialParseConfigFile(
		pctx,
		tgLogger,
		cfgfile,
		nil,
	)

	if err != nil {
		return nil, false, errors.E(ErrParsing, err, "parsing module at %s", cfgOpts.WorkingDir)
	}

	if tgConfig.Terraform == nil || tgConfig.Terraform.Source == nil {
		// not a runnable module
		return nil, false, nil
	}

	logger.Trace().Msgf("found terraform.source = %q", *tgConfig.Terraform.Source)

	stack, err := configstack.FindStackInSubfolders(context.Background(), tgLogger, cfgOpts)
	if err != nil {
		return nil, false, errors.E(err, "parsing module at %s", cfgOpts.WorkingDir)
	}

	modules := stack.Modules()
	if len(modules) == 0 {
		return nil, false, nil
	}

	var tgMod *configstack.TerraformModule
	for _, m := range modules {
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
			return nil, true, errors.E(err, "evaluating symlinks in %q", mod.Source)
		}
		// we normalize local paths as relative to the module.
		// so this is compatible with Terraform module sources.
		rel, err := filepath.Rel(cfgOpts.WorkingDir, src)
		if err != nil {
			return nil, true, errors.E(err, "normalizing local path %q", mod.Source)
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
			// ConfigPath is now a cty.Value in v0.82.0+
			if dep.ConfigPath.IsNull() {
				logger.Warn().Msg("dependency ConfigPath is null, skipping")
				continue
			}
			if !dep.ConfigPath.IsKnown() {
				logger.Warn().Msg("dependency ConfigPath is unknown, skipping")
				continue
			}
			if dep.ConfigPath.Type() != cty.String {
				logger.Warn().
					Str("type", dep.ConfigPath.Type().FriendlyName()).
					Msg("dependency ConfigPath is not a string, skipping")
				continue
			}

			depConfigPath := dep.ConfigPath.AsString()
			depAbsPath := depConfigPath
			if !filepath.IsAbs(depConfigPath) {
				depAbsPath = filepath.Join(tgMod.Path, depConfigPath)
			}

			logger.Trace().
				Str("mod-path", tgMod.Path).
				Str("dep-path", depConfigPath).
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
		mod.FilesProcessed[dependsAbsPath] = struct{}{}
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

	return mod, true, nil
}

// ScanModules scans dir looking for Terragrunt modules. It returns a list of
// modules with its "DependsOn paths" computed.
func ScanModules(rootdir string, dir project.Path, trackDependencies bool) (Modules, error) {
	absDir := project.AbsPath(rootdir, dir.String())
	opts := newTerragruntOptions(absDir, "")

	tgConfigFiles, err := findConfigFilesInPath(absDir, opts)
	if err != nil {
		return nil, errors.E(err, "scanning Terragrunt modules")
	}

	logger := log.With().
		Str("action", "tg.ScanModules").
		Logger()

	var modules Modules

	sort.Strings(tgConfigFiles)

	// Use a shared cache for all modules in this scan to avoid re-parsing the same files.
	// Returns nil if caching is disabled via TM_DISABLE_TG_CACHE.
	cache := getDefaultParseCache()

	fileErrs := map[string]*errors.List{}
	fileProcessed := map[string]struct{}{}
	for _, absCfgFile := range tgConfigFiles {
		fileErrs[absCfgFile] = errors.L()
		logger := logger.With().Str("cfg-file", absCfgFile).Logger()

		logger.Trace().Msg("found configuration")

		fileName := filepath.Base(absCfgFile)
		fileDir := filepath.Dir(absCfgFile)
		dir := project.PrjAbsPath(rootdir, fileDir)

		mod, isRootModule, err := LoadModuleWithCache(rootdir, dir, fileName, trackDependencies, cache)
		if err != nil {
			fileErrs[absCfgFile].Append(err)
			continue
		}

		if !isRootModule {
			continue
		}

		for file := range mod.FilesProcessed {
			fileProcessed[file] = struct{}{}
		}

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

func newTerragruntOptions(dir string, cfgfile string) *options.TerragruntOptions {
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

	// Logger is set separately when creating ParsingContext in v0.82.0+

	opts.DownloadDir = util.JoinPath(opts.WorkingDir, util.TerragruntCacheDir)
	opts.TerragruntConfigPath = cfgfile

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
