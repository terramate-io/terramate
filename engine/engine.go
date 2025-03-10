// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package engine

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/rs/zerolog/log"
	hhcl "github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cmd/terramate/cli/cliconfig"
	"github.com/terramate-io/terramate/cmd/terramate/cli/tmcloud/auth"
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
	prj "github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/stack"
	"github.com/terramate-io/terramate/stdlib"
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
	Engine struct {
		project *Project
		usercfg cliconfig.Config

		httpClient http.Client
		state      state

		printers printer.Printers

		uimode UIMode
	}

	GitFilter struct {
		IsChanged  bool
		ChangeBase string

		EnableUntracked   *bool
		EnableUncommitted *bool
	}

	state struct {
		stdout         io.Writer
		stderr         io.Writer
		stdin          io.Reader
		affectedStacks config.List[stack.Entry]

		cloud cloudState
	}
)

func NoGitFilter() GitFilter { return GitFilter{} }

func Load(wd string, uimode UIMode, printers printer.Printers) (e *Engine, found bool, err error) {
	prj, found, err := NewProject(wd)
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
	if prj.isRepo {
		prj.setupGitValues()
	}

	return &Engine{
		project:  prj,
		printers: printers,
		uimode:   uimode,
	}, true, nil
}

func (e *Engine) wd() string                   { return e.project.wd }
func (e *Engine) rootdir() string              { return e.project.rootdir }
func (e *Engine) BaseRef() string              { return e.project.baseRef }
func (e *Engine) stackManager() *stack.Manager { return e.project.stackManager }
func (e *Engine) RootNode() hcl.Config         { return e.project.root.Tree().Node }
func (e *Engine) cred() auth.Credential        { return e.state.cloud.client.Credential.(auth.Credential) }
func (e *Engine) cloudRegion() cloud.Region {
	rootcfg := e.RootNode()
	if rootcfg.Terramate != nil && rootcfg.Terramate.Config != nil && rootcfg.Terramate.Config.Cloud != nil {
		return rootcfg.Terramate.Config.Cloud.Location
	}
	return cloud.EU
}

func (e *Engine) Config() *config.Root {
	return e.project.root
}

func (e *Engine) SetConfig(r *config.Root) {
	e.project.root = r
}

func (e *Engine) Project() *Project { return e.project }

func (e *Engine) StackManager() *stack.Manager { return e.project.stackManager }

func (e *Engine) ListStacks(gitfilter GitFilter, target string, stackFilters cloud.StatusFilters, checkRepo bool) (*stack.Report, error) {
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
		cloudStacks, err := e.state.cloud.client.StacksByStatus(ctx, e.state.cloud.run.orgUUID, repository.Repo, target, stackFilters)
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

	e.project.Git.RepoChecks = report.Checks
	return report, nil
}

func (e *Engine) ComputeSelectedStacks(gitfilter GitFilter, tags filter.TagClause, ensureCleanRepo bool, outputFlags OutputsSharingOptions, target string, stackFilters cloud.StatusFilters) (config.List[*config.SortableStack], error) {
	report, err := e.ListStacks(gitfilter, target, stackFilters, true)
	if err != nil {
		return nil, err
	}

	entries := e.FilterStacks(report.Stacks, tags)
	stacks := make(config.List[*config.SortableStack], len(entries))
	for i, e := range entries {
		stacks[i] = e.Stack.Sortable()
	}

	stacks, err = e.stackManager().AddWantedOf(stacks)
	if err != nil {
		return nil, errors.E(err, "adding wanted stacks")
	}
	return e.addOutputDependencies(outputFlags, stacks)
}

func (e *Engine) addOutputDependencies(outputFlags OutputsSharingOptions, stacks config.List[*config.SortableStack]) (config.List[*config.SortableStack], error) {
	logger := log.With().
		Str("action", "engine.addOutputDependencies()").
		Logger()

	if !outputFlags.IncludeOutputDependencies && !outputFlags.OnlyOutputDependencies {
		logger.Debug().Msg("output dependencies not requested")
		return stacks, nil
	}

	if outputFlags.IncludeOutputDependencies && outputFlags.OnlyOutputDependencies {
		return nil, errors.E("--include-output-dependencies and --only-output-dependencies cannot be used together")
	}
	if (outputFlags.IncludeOutputDependencies || outputFlags.OnlyOutputDependencies) && !e.Config().HasExperiment(hcl.SharingIsCaringExperimentName) {
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
		evalctx, err := e.setupEvalContext(st.Stack, map[string]string{})
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

	if outputFlags.IncludeOutputDependencies {
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

func (e *Engine) FilterStacks(stacks []stack.Entry, tags filter.TagClause) []stack.Entry {
	return e.filterStacksByTags(e.filterStacksByWorkingDir(stacks), tags)
}

func (e *Engine) filterStacksByBasePath(basePath project.Path, stacks []stack.Entry) []stack.Entry {
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
	return filtered
}

func (e *Engine) filterStacksByWorkingDir(stacks []stack.Entry) []stack.Entry {
	rootdir := e.Config().HostDir()
	return e.filterStacksByBasePath(project.PrjAbsPath(rootdir, e.project.wd), stacks)
}

func (c *Engine) filterStacksByTags(entries []stack.Entry, tags filter.TagClause) []stack.Entry {
	if tags.IsEmpty() {
		return entries
	}
	filtered := []stack.Entry{}
	for _, entry := range entries {
		if filter.MatchTags(tags, entry.Stack.Tags) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

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
		} else {
			remoteCheckFailed = true
		}
	}

	if gitfilter.ChangeBase != "" {
		e.project.baseRef = gitfilter.ChangeBase
	} else if remoteCheckFailed {
		e.project.baseRef = e.project.defaultLocalBaseRef()
	} else {
		e.project.baseRef = e.project.defaultBaseRef()
	}
	return nil
}

func (e *Engine) setupEvalContext(st *config.Stack, overrideGlobals map[string]string) (*eval.Context, error) {
	runtime := e.Config().Runtime()

	if e.state.cloud.run.target != "" {
		runtime["target"] = cty.StringVal(e.state.cloud.run.target)
	}

	var tdir string
	if st != nil {
		tdir = st.HostDir(e.Config())
		runtime.Merge(st.RuntimeValues(e.Config()))
	} else {
		tdir = e.wd()
	}

	ctx := eval.NewContext(stdlib.NoFS(tdir, e.RootNode().Experiments()))
	ctx.SetNamespace("terramate", runtime)

	wdPath := prj.PrjAbsPath(e.rootdir(), tdir)
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

	e.state.cloud.run.target = targetArg
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
