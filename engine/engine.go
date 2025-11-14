// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package engine provides the core functionality of Terramate.
package engine

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/rs/zerolog/log"
	hhcl "github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/terramate/cloud/api/resources"
	cloudstack "github.com/terramate-io/terramate/cloud/api/stack"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/config/filter"
	"github.com/terramate-io/terramate/config/tag"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/globals"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/hcl/info"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/stack"
	"github.com/terramate-io/terramate/stdlib"
	"github.com/terramate-io/terramate/ui/tui/cliconfig"
	"github.com/terramate-io/terramate/ui/tui/clitest"
	"github.com/zclconf/go-cty/cty"
)

const (
	cloudFeatStatus          = "--status' is a Terramate Cloud feature to filter stacks that failed to deploy or have drifted."
	cloudFeatSyncDeployment  = "'--sync-deployment' is a Terramate Cloud feature to synchronize deployment details to Terramate Cloud."
	cloudFeatSyncDriftStatus = "'--sync-drift-status' is a Terramate Cloud feature to synchronize drift and health check results to Terramate Cloud."
	cloudFeatSyncPreview     = "'--sync-preview' is a Terramate Cloud feature to synchronize deployment previews to Terramate Cloud."
)

const targetIDRegexPattern = "^[a-z0-9][-_a-z0-9]*[a-z0-9]$"

var targetIDRegex = regexp.MustCompile(targetIDRegexPattern)

const (
	// HumanMode is the default normal mode when Terramate is executed at the user's machine.
	HumanMode UIMode = iota
	// AutomationMode is the mode when Terramate executes in the CI/CD environment.
	AutomationMode
)

// UIMode defines different modes of operation for the cli.
type UIMode int

type (
	// Engine holds the Terramate runtime state and does the heavy lifting of the CLI.
	// The engine exposes an API for the core machinery of Terramate (stack management,
	// change detection, stack ordering, etc) and is used by the CLI commands.
	// Note(i4k): It's stateful and shared between commands, so beware of the side-effects
	// of calling its methods. This is not the ideal design but the result of refactoring
	// the cli package into per-package commands.
	Engine struct {
		project *Project
		hclOpts []hcl.Option
		usercfg cliconfig.Config

		HTTPClient http.Client
		state      state

		printers  printer.Printers
		verbosity int

		uimode UIMode

		loadTerragruntModules bool
	}

	// GitFilter holds the configuration for git change detection.
	GitFilter struct {
		IsChanged  bool
		ChangeBase string

		EnableUntracked   *bool
		EnableUncommitted *bool
	}

	state struct {
		affectedStacks config.List[stack.Entry]
		repoChecks     stack.RepoChecks

		cloud CloudState
	}

	// DependencyFilters holds the configuration for filtering stacks based on their dependencies
	// and dependents.
	DependencyFilters struct {
		// Deprecated: Use IncludeDependencies instead
		IncludeOutputDependencies bool
		// Deprecated: Use OnlyDependencies instead
		OnlyOutputDependencies bool

		// Dependency filters (what selected stacks depend on)
		IncludeAllDependencies    bool
		IncludeDirectDependencies bool
		OnlyAllDependencies       bool
		OnlyDirectDependencies    bool
		ExcludeAllDependencies    bool

		// Dependent filters (what depends on selected stacks)
		IncludeAllDependents    bool
		IncludeDirectDependents bool
		OnlyDirectDependents    bool
		OnlyAllDependents       bool
		ExcludeAllDependents    bool
	}
)

// HasDependencyFilters returns if there are any dependency filters set.
func (df *DependencyFilters) HasDependencyFilters() bool {
	return df.IncludeAllDependencies || df.IncludeDirectDependencies || df.OnlyAllDependencies || df.OnlyDirectDependencies || df.ExcludeAllDependencies ||
		df.IncludeAllDependents || df.IncludeDirectDependents ||
		df.OnlyDirectDependents || df.OnlyAllDependents || df.ExcludeAllDependents
}

// NoGitFilter returns a GitFilter for unfiltered list.
func NoGitFilter() GitFilter { return GitFilter{} }

// Load loads the engine with the given working directory and CLI configuration.
// If the project is not found, it returns false.
func Load(wd string, loadTerragruntModules bool, clicfg cliconfig.Config, uimode UIMode, printers printer.Printers, verbosity int, hclOpts ...hcl.Option) (e *Engine, found bool, err error) {
	prj, found, err := NewProject(wd, loadTerragruntModules, hclOpts...)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}
	err = prj.setDefaults()
	if err != nil {
		return nil, true, errors.E(err, "setting configuration")
	}
	return &Engine{
		project:               prj,
		printers:              printers,
		verbosity:             verbosity,
		uimode:                uimode,
		usercfg:               clicfg,
		hclOpts:               hclOpts,
		loadTerragruntModules: loadTerragruntModules,
	}, true, nil
}

func (e *Engine) wd() string                   { return e.project.wd }
func (e *Engine) rootdir() string              { return e.project.rootdir }
func (e *Engine) stackManager() *stack.Manager { return e.project.stackManager }

// BaseRef returns the git baseref of the project.
func (e *Engine) BaseRef() string { return e.project.baseRef }

// RepoChecks returns the cached repository checks.
func (e *Engine) RepoChecks() stack.RepoChecks { return e.state.repoChecks }

// RootNode returns the root node of the project.
func (e *Engine) RootNode() hcl.Config { return e.project.root.Tree().Node }

// Config returns the root configuration of the project.
func (e *Engine) Config() *config.Root {
	return e.project.root
}

// HCLOptions returns the HCL options used to load the configuration.
func (e *Engine) HCLOptions() []hcl.Option {
	return e.hclOpts
}

// ReloadConfig reloads the root configuration of the project.
func (e *Engine) ReloadConfig() error {
	rootcfg, err := config.LoadRoot(e.rootdir(), e.loadTerragruntModules, e.hclOpts...)
	if err != nil {
		return err
	}
	e.project.root = rootcfg
	return nil
}

// CLIConfig returns the CLI configuration.
func (e *Engine) CLIConfig() cliconfig.Config {
	return e.usercfg
}

// Project returns the project.
func (e *Engine) Project() *Project { return e.project }

// StackManager returns the stack manager.
func (e *Engine) StackManager() *stack.Manager { return e.project.stackManager }

// ListStacks returns the list of stacks based on filters.
func (e *Engine) ListStacks(gitfilter GitFilter, target string, stackFilters resources.StatusFilters, checkRepo bool) (*stack.Report, error) {
	var report *stack.Report

	err := e.setupGit(gitfilter)
	if err != nil {
		return nil, err
	}

	mgr := e.StackManager()
	if gitfilter.IsChanged {
		report, err = mgr.ListChanged(stack.ChangeConfig{
			BaseRef:            e.project.baseRef,
			UntrackedChanges:   gitfilter.EnableUntracked,
			UncommittedChanges: gitfilter.EnableUncommitted,
		})
	} else {
		report, err = mgr.List(checkRepo)
	}

	if err != nil {
		return nil, err
	}

	// memoize the list of affected stacks so they can be retrieved later
	// without computing the list again
	e.state.affectedStacks = report.Stacks

	if stackFilters.HasFilter() {
		if !e.project.isRepo {
			return nil, errors.E("cloud filters requires a git repository")
		}
		err := e.SetupCloudConfig([]string{cloudFeatStatus})
		if err != nil {
			return nil, err
		}

		repository, err := e.project.Repo()
		if err != nil {
			return nil, err
		}
		if repository.Host == "local" {
			return nil, errors.E("status filters does not work with filesystem based remotes")
		}

		ctx, cancel := context.WithTimeout(context.Background(), defaultCloudTimeout)
		defer cancel()
		cloudStacks, err := e.state.cloud.client.StacksByStatus(ctx, e.state.cloud.Org.UUID, repository.Repo, target, stackFilters)
		if err != nil {
			return nil, err
		}

		cloudStacksMap := map[string]bool{}
		for _, stack := range cloudStacks {
			cloudStacksMap[stack.MetaID] = true
		}

		localStacks := report.Stacks
		var stacks []stack.Entry

		for _, stack := range localStacks {
			if cloudStacksMap[strings.ToLower(stack.Stack.ID)] {
				stacks = append(stacks, stack)
			}
		}
		report.Stacks = stacks
	}

	e.state.repoChecks = report.Checks
	return report, nil
}

// ComputeSelectedStacks computes stacks based on filters, working directory, tags, filesystem ordering, git changes, etc.
func (e *Engine) ComputeSelectedStacks(gitfilter GitFilter, tags filter.TagClause, dependencyFilters DependencyFilters, target string, stackFilters resources.StatusFilters) (config.List[*config.SortableStack], error) {
	report, err := e.ListStacks(gitfilter, target, stackFilters, true)
	if err != nil {
		return nil, err
	}

	entries, err := e.FilterStacks(report.Stacks, ByWorkingDir(), ByTagsClause(tags))
	if err != nil {
		return nil, err
	}
	stacks := make(config.List[*config.SortableStack], len(entries))
	for i, e := range entries {
		stacks[i] = e.Stack.Sortable()
	}

	stacks, err = e.stackManager().AddWantedOf(stacks)
	if err != nil {
		return nil, errors.E(err, "adding wanted stacks")
	}
	return e.AddOutputDependencies(dependencyFilters, stacks, target)
}

// AddOutputDependencies takes a list of stacks and adds potential output dependencies.
// Deprecated: This method now supports all dependency filtering via DependencyFilters.
func (e *Engine) AddOutputDependencies(dependencyFilters DependencyFilters, stacks config.List[*config.SortableStack], target string) (config.List[*config.SortableStack], error) {
	return e.ApplyDependencyFilters(dependencyFilters, stacks, target)
}

// applyOutputSharingFilters implements the original deprecated behavior
// that only considers input.from_stack_id dependencies (output-sharing)
func (e *Engine) applyOutputSharingFilters(dependencyFilters DependencyFilters, stacks config.List[*config.SortableStack], target string) (config.List[*config.SortableStack], error) {
	logger := log.With().
		Str("action", "engine.applyOutputSharingFilters()").
		Logger()

	if !dependencyFilters.IncludeOutputDependencies && !dependencyFilters.OnlyOutputDependencies {
		logger.Debug().Msg("output dependencies not requested")
		return stacks, nil
	}

	if dependencyFilters.IncludeOutputDependencies && dependencyFilters.OnlyOutputDependencies {
		return nil, errors.E("--include-output-dependencies and --only-output-dependencies cannot be used together")
	}
	if (dependencyFilters.IncludeOutputDependencies || dependencyFilters.OnlyOutputDependencies) && !e.Config().HasExperiment(hcl.SharingIsCaringExperimentName) {
		return nil, errors.E("--include-output-dependencies requires the '%s' experiment enabled", hcl.SharingIsCaringExperimentName)
	}

	stacksMap := map[string]*config.SortableStack{}
	for _, stack := range stacks {
		stacksMap[stack.Stack.Dir.String()] = stack
	}

	rootcfg := e.Config()
	depIDs := map[string]struct{}{}
	depOrigins := map[string][]string{} // id -> stack paths
	for _, st := range stacks {
		evalctx, err := e.SetupEvalContext(e.wd(), st.Stack, target, map[string]string{})
		if err != nil {
			return nil, err
		}
		cfg, _ := rootcfg.Lookup(st.Stack.Dir)
		for _, inputcfg := range cfg.Node.Inputs {
			fromStackID, err := config.EvalInputFromStackID(evalctx, inputcfg)
			if err != nil {
				return nil, errors.E(err, "evaluating `input.%s.from_stack_id`", inputcfg.Name)
			}
			depIDs[fromStackID] = struct{}{}
			depOrigins[fromStackID] = append(depOrigins[fromStackID], st.Stack.Dir.String())

			logger.Debug().
				Str("stack", st.Stack.Dir.String()).
				Str("dependency", fromStackID).
				Msg("stack has output dependency")
		}
	}

	mgr := e.stackManager()
	outputsMap := map[string]*config.SortableStack{}
	for depID := range depIDs {
		st, found, err := mgr.StackByID(depID)
		if err != nil {
			return nil, errors.E(err, "loading output dependencies of selected stacks")
		}
		if !found {
			return nil, errors.E(
				errors.E("dependency stack %s not found", depID),
				"loading output dependencies of selected stacks")
		}

		var reason string
		depsOf := depOrigins[depID]
		if len(depsOf) == 1 {
			reason = fmt.Sprintf("Output dependency of stack %s", depsOf[0])
		} else {
			reason = fmt.Sprintf("Output dependency of stacks %s", strings.Join(depsOf, ", "))
		}

		logger.Debug().
			Str("stack", st.Dir.String()).
			Str("reason", reason).
			Msg("adding output dependency")

		outputsMap[st.Dir.String()] = &config.SortableStack{
			Stack: st,
		}
	}

	if dependencyFilters.IncludeOutputDependencies {
		for _, dep := range outputsMap {
			if _, found := stacksMap[dep.Stack.Dir.String()]; !found {
				stacks = append(stacks, dep)
			}
		}
		return stacks, nil
	}

	// only output dependencies
	stacks = config.List[*config.SortableStack]{}
	for _, dep := range outputsMap {
		stacks = append(stacks, dep)
	}
	return stacks, nil
}

// ApplyDependencyFilters applies dependency and dependent filters to the given stacks.
// It supports both legacy output-sharing filters and new comprehensive dependency filters
// for both Terragrunt and Terramate native stacks.
func (e *Engine) ApplyDependencyFilters(opts DependencyFilters, stacks config.List[*config.SortableStack], target string) (config.List[*config.SortableStack], error) {
	logger := log.With().
		Str("action", "engine.ApplyDependencyFilters()").
		Logger()

	// Handle deprecated flags with original logic (output-sharing only)
	if opts.IncludeOutputDependencies || opts.OnlyOutputDependencies {
		if !e.Config().HasExperiment(hcl.SharingIsCaringExperimentName) {
			return nil, errors.E("--include-output-dependencies requires the '%s' experiment enabled", hcl.SharingIsCaringExperimentName)
		}

		if opts.IncludeOutputDependencies && opts.OnlyOutputDependencies {
			return nil, errors.E("--include-output-dependencies and --only-output-dependencies cannot be used together")
		}

		if opts.IncludeOutputDependencies {
			logger.Warn().Msg("--include-output-dependencies is deprecated, use --include-direct-dependencies instead")
		}
		if opts.OnlyOutputDependencies {
			logger.Warn().Msg("--only-output-dependencies is deprecated, use --only-direct-dependencies instead")
		}

		// Use original output-sharing logic to preserve exact same behavior
		return e.applyOutputSharingFilters(opts, stacks, target)
	}

	// Check if any filters are requested
	if !opts.HasDependencyFilters() {
		logger.Debug().Msg("no dependency filters requested")
		return stacks, nil
	}

	// Validate filter combinations
	if err := e.validateDependencyFilterOptions(opts); err != nil {
		return nil, err
	}

	// Load all stacks for dependency graph construction
	allStacks, err := config.LoadAllStacks(e.Config(), e.Config().Tree())
	if err != nil {
		return nil, errors.E(err, "loading all stacks for dependency graph")
	}

	// Build dependency graph
	graph, err := e.NewDependencyGraph(allStacks, target)
	if err != nil {
		return nil, errors.E(err, "building dependency graph")
	}

	// Check for cycles and warn
	cycles := graph.DetectCycles()
	if len(cycles) > 0 {
		logger.Warn().
			Int("cycle_count", len(cycles)).
			Msg("circular dependencies detected in stack graph")
		for i, cycle := range cycles {
			logger.Warn().
				Int("cycle_num", i+1).
				Strs("stacks", cycle).
				Msg("circular dependency")
		}
	}

	// Get current stack paths
	currentStackPaths := make([]string, len(stacks))
	stacksMap := make(map[string]*config.SortableStack)
	for i, st := range stacks {
		path := st.Stack.Dir.String()
		currentStackPaths[i] = path
		stacksMap[path] = st
	}

	// Apply filters in order: only → include → exclude
	resultPaths := make(map[string]bool)

	// Step 1: Handle "only" filters (replace selection)
	if opts.OnlyAllDependencies {
		logger.Debug().Msg("applying only-all-dependencies filter")
		deps := graph.GetAllDependenciesForStacks(currentStackPaths)
		for _, dep := range deps {
			resultPaths[dep] = true
		}
	} else if opts.OnlyDirectDependencies {
		logger.Debug().Msg("applying only-direct-dependencies filter")
		deps := graph.GetDirectDependenciesForStacks(currentStackPaths)
		for _, dep := range deps {
			resultPaths[dep] = true
		}
	} else if opts.OnlyDirectDependents {
		logger.Debug().Msg("applying only-direct-dependents filter")
		deps := graph.GetDirectDependentsForStacks(currentStackPaths)
		for _, dep := range deps {
			resultPaths[dep] = true
		}
	} else if opts.OnlyAllDependents {
		logger.Debug().Msg("applying only-all-dependents filter")
		deps := graph.GetAllDependentsForStacks(currentStackPaths)
		for _, dep := range deps {
			resultPaths[dep] = true
		}
	} else {
		// No "only" filter, start with current selection
		for _, path := range currentStackPaths {
			resultPaths[path] = true
		}
	}

	// Step 2: Handle "include" filters (add to selection)
	if opts.IncludeAllDependencies {
		logger.Debug().Msg("applying include-all-dependencies filter")
		deps := graph.GetAllDependenciesForStacks(currentStackPaths)
		for _, dep := range deps {
			resultPaths[dep] = true
		}
	}
	if opts.IncludeDirectDependencies {
		logger.Debug().Msg("applying include-direct-dependencies filter")
		deps := graph.GetDirectDependenciesForStacks(currentStackPaths)
		for _, dep := range deps {
			resultPaths[dep] = true
		}
	}
	if opts.IncludeDirectDependents {
		logger.Debug().Msg("applying include-direct-dependents filter")
		deps := graph.GetDirectDependentsForStacks(currentStackPaths)
		for _, dep := range deps {
			resultPaths[dep] = true
		}
	}
	if opts.IncludeAllDependents {
		logger.Debug().Msg("applying include-all-dependents filter")
		deps := graph.GetAllDependentsForStacks(currentStackPaths)
		for _, dep := range deps {
			resultPaths[dep] = true
		}
	}

	// Step 3: Handle "exclude" filters (remove from selection)
	// Extract current selection paths from resultPaths for exclude calculations
	currentResultPaths := make([]string, 0, len(resultPaths))
	for path := range resultPaths {
		currentResultPaths = append(currentResultPaths, path)
	}

	if opts.ExcludeAllDependencies {
		logger.Debug().Msg("applying exclude-all-dependencies filter")
		deps := graph.GetAllDependenciesForStacks(currentResultPaths)
		for _, dep := range deps {
			delete(resultPaths, dep)
		}
	}
	if opts.ExcludeAllDependents {
		logger.Debug().Msg("applying exclude-all-dependents filter")
		deps := graph.GetAllDependentsForStacks(currentResultPaths)
		for _, dep := range deps {
			delete(resultPaths, dep)
		}
	}

	// Build result list with consistent ordering
	result := config.List[*config.SortableStack]{}
	stacksByPath := make(map[string]*config.SortableStack)
	for _, st := range allStacks {
		stacksByPath[st.Stack.Dir.String()] = st
	}

	// Convert map to sorted slice for predictable output
	sortedPaths := make([]string, 0, len(resultPaths))
	for path := range resultPaths {
		sortedPaths = append(sortedPaths, path)
	}
	slices.Sort(sortedPaths)

	for _, path := range sortedPaths {
		if st, found := stacksByPath[path]; found {
			result = append(result, st)
		} else {
			logger.Warn().
				Str("path", path).
				Msg("stack in filter result not found in all stacks")
		}
	}

	logger.Debug().
		Int("input_count", len(stacks)).
		Int("output_count", len(result)).
		Msg("dependency filters applied")

	return result, nil
}

// validateDependencyFilterOptions validates that filter combinations are valid
func (e *Engine) validateDependencyFilterOptions(opts DependencyFilters) error {
	// Count "only" filters
	onlyCount := 0
	if opts.OnlyAllDependencies {
		onlyCount++
	}
	if opts.OnlyDirectDependencies {
		onlyCount++
	}
	if opts.OnlyDirectDependents {
		onlyCount++
	}
	if opts.OnlyAllDependents {
		onlyCount++
	}

	if onlyCount > 1 {
		return errors.E("only one of --only-all-dependencies, --only-direct-dependencies, --only-direct-dependents, or --only-all-dependents can be used at a time")
	}

	// "only" and "include" of the same type is redundant but not an error
	// "only" and "exclude" is valid (only X, but exclude Y subset)

	return nil
}

// FilterStacks filters stacks based on tags and working directory.
func (e *Engine) FilterStacks(stacks []stack.Entry, opts ...FilterStacksOption) ([]stack.Entry, error) {
	if len(opts) == 0 {
		return nil, errors.E(errors.ErrInternal, "FilterStacks called without any filters")
	}

	var err error
	for _, opt := range opts {
		stacks, err = opt(e, stacks)
		if err != nil {
			return nil, err
		}
	}
	return stacks, nil
}

// FilterStacksOption is an option for FilterStacks.
type FilterStacksOption func(e *Engine, stacks []stack.Entry) ([]stack.Entry, error)

// ByWorkingDir filters out stacks based on the current working directory.
func ByWorkingDir() FilterStacksOption {
	return func(e *Engine, stacks []stack.Entry) ([]stack.Entry, error) {
		rootdir := e.Config().HostDir()
		return ByBasePath(project.PrjAbsPath(rootdir, e.project.wd))(e, stacks)
	}
}

// ByBasePath filters out stacks not inside the given base path.
func ByBasePath(basePath project.Path) FilterStacksOption {
	return func(_ *Engine, stacks []stack.Entry) ([]stack.Entry, error) {
		baseStr := basePath.String()
		if baseStr != "/" {
			baseStr += "/"
		}
		filtered := []stack.Entry{}
		for _, e := range stacks {
			stackdir := e.Stack.Dir.String()
			if stackdir != "/" {
				stackdir += "/"
			}
			if strings.HasPrefix(stackdir, baseStr) {
				filtered = append(filtered, e)
			}
		}
		return filtered, nil
	}
}

// ByTagsClause filters stacks based on tags.
func ByTagsClause(tags filter.TagClause) FilterStacksOption {
	return func(_ *Engine, stacks []stack.Entry) ([]stack.Entry, error) {
		if tags.IsEmpty() {
			return stacks, nil
		}
		filtered := []stack.Entry{}
		for _, entry := range stacks {
			if filter.MatchTags(tags, entry.Stack.Tags) {
				filtered = append(filtered, entry)
			}
		}
		return filtered, nil
	}
}

// ByTags is a convenience wrapper for ByTagsClause that handles the tag parsing.
func ByTags(tags, notags []string) FilterStacksOption {
	return func(e *Engine, stacks []stack.Entry) ([]stack.Entry, error) {
		tagsClause, err := ParseFilterTags(tags, notags)
		if err != nil {
			return nil, err
		}
		return ByTagsClause(tagsClause)(e, stacks)
	}
}

// FriendlyFmtDir formats the directory path in a friendly way.
func (e *Engine) FriendlyFmtDir(dir string) (string, bool) {
	return project.FriendlyFmtDir(e.Config().HostDir(), e.project.wd, dir)
}

func (e *Engine) setupGit(gitfilter GitFilter) error {
	if !gitfilter.IsChanged || !e.project.IsGitFeaturesEnabled() {
		return nil
	}

	remoteCheckFailed := false
	if err := e.project.checkDefaultRemote(); err != nil {
		if e.project.Git.RemoteConfigured {
			return errors.E(err, "checking git default remote")
		}
		remoteCheckFailed = true
	}

	var err error
	if gitfilter.ChangeBase != "" {
		e.project.baseRef = gitfilter.ChangeBase
	} else if remoteCheckFailed {
		e.project.baseRef, err = e.project.DefaultLocalBaseRef()
	} else {
		e.project.baseRef, err = e.project.DefaultBaseRef()
	}
	if err != nil {
		return errors.E(err, "setting up git")
	}
	return nil
}

// SetupEvalContext sets up the evaluation context for a stack.
func (e *Engine) SetupEvalContext(wd string, st *config.Stack, target string, overrideGlobals map[string]string) (*eval.Context, error) {
	runtime := e.Config().Runtime()

	if target != "" {
		runtime["target"] = cty.StringVal(target)
	}

	var tdir string
	if st != nil {
		tdir = st.HostDir(e.Config())
		runtime.Merge(st.RuntimeValues(e.Config()))
	} else {
		tdir = wd
	}

	ctx := eval.NewContext(stdlib.NoFS(tdir, e.RootNode().Experiments()))
	ctx.SetNamespace("terramate", runtime)

	wdPath := project.PrjAbsPath(e.rootdir(), tdir)
	tree, ok := e.Config().Lookup(wdPath)
	if !ok {
		return nil, errors.E("Configuration at %s not found", wdPath)
	}
	exprs, err := globals.LoadExprs(tree)
	if err != nil {
		return nil, errors.E(err, "loading globals expressions")
	}

	for name, exprStr := range overrideGlobals {
		expr, err := ast.ParseExpression(exprStr, "<cmdline>")
		if err != nil {
			return nil, errors.E(err, "--global %s=%s is an invalid expresssion", name, exprStr)
		}
		parts := strings.Split(name, ".")
		length := len(parts)
		globalPath := globals.NewGlobalAttrPath(parts[0:length-1], parts[length-1])
		exprs.SetOverride(
			wdPath,
			globalPath,
			expr,
			info.NewRange(e.rootdir(), hhcl.Range{
				Filename: filepath.Join(e.rootdir(), "<cmdline>"),
				Start:    hhcl.InitialPos,
				End:      hhcl.InitialPos,
			}),
		)
	}
	_ = exprs.Eval(ctx)
	return ctx, nil
}

// DetectEvalContext detects the evaluation context for a stack.
func (e *Engine) DetectEvalContext(wd string, overrideGlobals map[string]string) (*eval.Context, error) {
	var st *config.Stack
	cfg := e.Config()
	if config.IsStack(cfg, wd) {
		var err error
		st, err = config.LoadStack(cfg, project.PrjAbsPath(cfg.HostDir(), wd))
		if err != nil {
			return nil, errors.E(err, "setup eval context: loading stack config")
		}
	}
	return e.SetupEvalContext(wd, st, "", overrideGlobals)
}

// ParseFilterTags parses the tags and notags arguments.
func ParseFilterTags(tags, notags []string) (filter.TagClause, error) {
	clauses, found, err := filter.ParseTagClauses(tags...)
	if err != nil {
		return filter.TagClause{}, errors.E(err, "unable to parse tag clauses")
	}
	var parsed filter.TagClause
	if found {
		parsed = clauses
	}

	for _, val := range notags {
		err := tag.Validate(val)
		if err != nil {
			return filter.TagClause{}, errors.E(err, "unable validate tag")
		}
	}
	var noClauses filter.TagClause
	if len(notags) == 0 {
		return parsed, nil
	}
	if len(notags) == 1 {
		noClauses = filter.TagClause{
			Op:  filter.NEQ,
			Tag: notags[0],
		}
	} else {
		var children []filter.TagClause
		for _, tagname := range notags {
			children = append(children, filter.TagClause{
				Op:  filter.NEQ,
				Tag: tagname,
			})
		}
		noClauses = filter.TagClause{
			Op:       filter.AND,
			Children: children,
		}
	}

	if parsed.IsEmpty() {
		parsed = noClauses
		return parsed, nil
	}

	switch parsed.Op {
	case filter.AND:
		parsed.Children = append(parsed.Children, noClauses)
	default:
		parsed = filter.TagClause{
			Op:       filter.AND,
			Children: []filter.TagClause{parsed, noClauses},
		}
	}
	return parsed, nil
}

// CheckTargetsConfiguration checks the target configuration of the project.
func (e *Engine) CheckTargetsConfiguration(targetArg, fromTargetArg string, cloudCheckFn func(bool) error) error {
	isTargetSet := targetArg != ""
	isFromTargetSet := fromTargetArg != ""
	isTargetsEnabled := e.Config().HasExperiment("targets") && e.Config().IsTargetsEnabled()

	if isTargetSet {
		if !isTargetsEnabled {
			printer.Stderr.Error(`The "targets" feature is not enabled`)
			printer.Stderr.Println(`In order to enable it you must set the terramate.config.experiments attribute and set terramate.config.cloud.targets.enabled to true.`)
			printer.Stderr.Println(`Example:
	
terramate {
  config {
    experiments = ["targets"]
    cloud {
      targets {
        enabled = true
      }
    }
  }
}`)
			os.Exit(1)
		}

		// Here we should check if any cloud parameter is enabled for target to make sense.
		// The error messages should be different per caller.
		err := cloudCheckFn(true)
		if err != nil {
			return err
		}

	} else {
		if isTargetsEnabled {
			// Here we should check if any cloud parameter is enabled that would require target.
			// The error messages should be different per caller.
			err := cloudCheckFn(false)
			if err != nil {
				return err
			}
		}
	}

	if isFromTargetSet && !isTargetSet {
		return errors.E("--from-target requires --target")
	}

	if isTargetSet && !targetIDRegex.MatchString(targetArg) {
		return errors.E("--target value has invalid format, it must match %q", targetIDRegexPattern)
	}

	if isFromTargetSet && !targetIDRegex.MatchString(fromTargetArg) {
		return errors.E("--from-target value has invalid format, it must match %q", targetIDRegexPattern)
	}

	return nil
}

// EnsureAllStackHaveIDs ensures all stacks have IDs.
func (e *Engine) EnsureAllStackHaveIDs(stacks config.List[*config.SortableStack]) error {
	logger := log.With().
		Str("action", "engine.ensureAllStackHaveIDs").
		Logger()

	var stacksMissingIDs []string
	for _, st := range stacks {
		if st.ID == "" {
			stacksMissingIDs = append(stacksMissingIDs, st.Dir().String())
		}
	}
	if len(stacksMissingIDs) > 0 {
		for _, stackPath := range stacksMissingIDs {
			logger.Error().Str("stack", stackPath).Msg("stack is missing the ID field")
		}
		logger.Warn().Msg("Stacks are missing IDs. You can use 'terramate create --ensure-stack-ids' to add missing IDs to all stacks.")
		return e.handleCriticalError(errors.E(clitest.ErrCloudStacksWithoutID))
	}
	return nil
}

func (e *Engine) handleCriticalError(err error) error {
	if err != nil {
		if e.uimode == HumanMode {
			return err
		}

		e.DisableCloudFeatures(err)
	}
	return nil
}

func checkChangeDetectionFlagConflicts(enable []string, disable []string) error {
	for _, enableOpt := range enable {
		if slices.Contains(disable, enableOpt) {
			return errors.E("conflicting option %s in --{enable,disable}-change-detection flags", enableOpt)
		}
	}
	return nil
}

// NewGitFilter creates a new GitFilter.
func NewGitFilter(isChanged bool, gitChangeBase string, enable []string, disable []string) (GitFilter, error) {
	err := checkChangeDetectionFlagConflicts(enable, disable)
	if err != nil {
		return GitFilter{}, err
	}

	on := true
	off := false

	filter := GitFilter{
		IsChanged:  isChanged,
		ChangeBase: gitChangeBase,
	}

	if slices.Contains(enable, "git-untracked") {
		filter.EnableUntracked = &on
	}
	if slices.Contains(enable, "git-uncommitted") {
		filter.EnableUncommitted = &on
	}
	if slices.Contains(disable, "git-untracked") {
		filter.EnableUntracked = &off
	}
	if slices.Contains(disable, "git-uncommitted") {
		filter.EnableUncommitted = &off
	}
	return filter, nil
}

// GetAffectedStacks returns the list of stacks affected by the current command.
// c.affectedStacks is expected to be already set, if not it will be computed
// and cached.
func (e *Engine) GetAffectedStacks(gitfilter GitFilter) ([]stack.Entry, error) {
	if e.state.affectedStacks != nil {
		return e.state.affectedStacks, nil
	}
	report, err := e.ListStacks(gitfilter, cloudstack.AnyTarget, resources.NoStatusFilters(), false)
	if err != nil {
		return nil, err
	}

	e.state.affectedStacks = report.Stacks
	return e.state.affectedStacks, nil
}
