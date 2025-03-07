package engine

import (
	"context"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cmd/terramate/cli/cliconfig"
	"github.com/terramate-io/terramate/cmd/terramate/cli/tmcloud/auth"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/config/filter"
	"github.com/terramate-io/terramate/config/tag"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/stack"
)

const (
	cloudFeatStatus          = "--status' is a Terramate Cloud feature to filter stacks that failed to deploy or have drifted."
	cloudFeatSyncDeployment  = "'--sync-deployment' is a Terramate Cloud feature to synchronize deployment details to Terramate Cloud."
	cloudFeatSyncDriftStatus = "'--sync-drift-status' is a Terramate Cloud feature to synchronize drift and health check results to Terramate Cloud."
	cloudFeatSyncPreview     = "'--sync-preview' is a Terramate Cloud feature to synchronize deployment previews to Terramate Cloud."
)

const targetIDRegexPattern = "^[a-z0-9][-_a-z0-9]*[a-z0-9]$"

var targetIDRegex = regexp.MustCompile(targetIDRegexPattern)

type (
	Engine struct {
		project *Project
		usercfg cliconfig.Config

		httpClient http.Client
		state      state

		printers printer.Printers
	}

	GitFilter struct {
		IsChanged  bool
		ChangeBase string

		EnableUntracked   *bool
		EnableUncommitted *bool
	}

	state struct {
		affectedStacks config.List[stack.Entry]

		cloud cloudState
	}
)

func NoGitFilter() GitFilter { return GitFilter{} }

func Load(wd string, printers printer.Printers) (e *Engine, found bool, err error) {
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
	}, true, nil
}

func (e *Engine) wd() string                   { return e.project.wd }
func (e *Engine) rootdir() string              { return e.project.rootdir }
func (e *Engine) cfg() *config.Root            { return e.project.root }
func (e *Engine) baseRef() string              { return e.project.baseRef }
func (e *Engine) stackManager() *stack.Manager { return e.project.stackManager }
func (e *Engine) rootNode() hcl.Config         { return e.project.root.Tree().Node }
func (e *Engine) cred() auth.Credential        { return e.state.cloud.client.Credential.(auth.Credential) }
func (e *Engine) cloudRegion() cloud.Region {
	rootcfg := e.rootNode()
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
		err := e.setupCloudConfig([]string{cloudFeatStatus})
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

	e.project.git.repoChecks = report.Checks
	return report, nil
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
	if !gitfilter.IsChanged || !e.project.isGitFeaturesEnabled() {
		return nil
	}

	remoteCheckFailed := false
	if err := e.project.checkDefaultRemote(); err != nil {
		if e.project.git.remoteConfigured {
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
