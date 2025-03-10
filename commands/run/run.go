// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package run

import (
	"context"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cmd/terramate/cli/clitest"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/safeguard"
	"github.com/terramate-io/terramate/stack"
)

const (
	// ErrRunFailed represents the error when the execution fails, whatever the reason.
	ErrRunFailed errors.Kind = "execution failed"

	// ErrRunCanceled represents the error when the execution was canceled.
	ErrRunCanceled errors.Kind = "execution canceled"

	// ErrRunCommandNotExecuted represents the error when the command was not executed for whatever reason.
	ErrRunCommandNotExecuted errors.Kind = "command not found"

	// ErrCurrentHeadIsOutOfDate indicates the local HEAD revision is outdated.
	ErrCurrentHeadIsOutOfDate errors.Kind = "current HEAD is out-of-date with the remote base branch"
	// ErrOutdatedGenCodeDetected indicates outdated generated code detected.
	ErrOutdatedGenCodeDetected errors.Kind = "outdated generated code detected"

	cloudSyncPreviewCICDWarning = "--sync-preview is only supported in GitHub Actions workflows, Gitlab CICD pipelines or Bitbucket Cloud Pipelines"
)

type Spec struct {
	Engine     *engine.Engine
	WorkingDir string
	Printers   printer.Printers
	Stdout     io.Writer
	Stderr     io.Writer
	Stdin      io.Reader

	// Behavior control options
	Command         []string
	Quiet           bool
	DryRun          bool
	Reverse         bool
	ScriptRun       bool
	ContinueOnError bool
	Parallel        int
	NoRecursive     bool

	SyncDeployment  bool
	SyncDriftStatus bool
	SyncPreview     bool

	StatusFilters StatusFilters
	Target        string
	Tags          []string
	NoTags        []string

	engine.OutputsSharingOptions

	Safeguards Safeguards
}

type StatusFilters struct {
	StackStatus      string
	DeploymentStatus string
	DriftStatus      string
}

type Safeguards struct {
	DisableCheckGitUntracked          bool
	DisableCheckGitUncommitted        bool
	DisableCheckGitRemote             bool
	DisableCheckGenerateOutdatedCheck bool

	reEnabled bool
}

func (s *Spec) Name() string { return "run" }

func (s *Spec) Exec(ctx context.Context) error {
	// TODO(i4k): setup safegaurds!!!
	err := gitSafeguardDefaultBranchIsReachable(s.Engine, s.Safeguards)
	if err != nil {
		return err
	}

	if len(s.Command) == 0 {
		return errors.E("run expects a command")
	}

	s.checkOutdatedGeneratedCode()
	s.checkCloudSync()

	cfg := s.Engine.Config()
	rootdir := cfg.HostDir()
	var stacks config.List[*config.SortableStack]
	if s.NoRecursive {
		st, found, err := config.TryLoadStack(cfg, project.PrjAbsPath(rootdir, s.WorkingDir))
		if err != nil {
			return errors.E(err, "loading stack in current directory")
		}

		if !found {
			return errors.E("--no-recursive provided but no stack found in the current directory")
		}

		stacks = append(stacks, st.Sortable())
	} else {
		cloudFilters, err := cloud.ParseStatusFilters(
			s.StatusFilters.StackStatus,
			s.StatusFilters.DeploymentStatus,
			s.StatusFilters.DriftStatus,
		)
		if err != nil {
			return err
		}
		tags, err := engine.ParseFilterTags(s.Tags, s.NoTags)
		if err != nil {
			return err
		}
		stacks, err = s.Engine.ComputeSelectedStacks(gitfilter, tags, true, s.OutputsSharingOptions, s.Target, cloud.StatusFilters{
			StackStatus:      stackFilter,
			DeploymentStatus: deploymentFilter,
			DriftStatus:      driftFilter,
		})
		if err != nil {
			fatal(err)
		}
	}

	if c.parsedArgs.Run.SyncDeployment && c.parsedArgs.Run.SyncDriftStatus {
		fatal("--sync-deployment conflicts with --sync-drift-status")
	}

	if c.parsedArgs.Run.SyncPreview && (c.parsedArgs.Run.SyncDeployment || c.parsedArgs.Run.SyncDriftStatus) {
		fatal("cannot use --sync-preview with --sync-deployment or --sync-drift-status")
	}

	if c.parsedArgs.Run.TerraformPlanFile != "" && c.parsedArgs.Run.TofuPlanFile != "" {
		fatal("--terraform-plan-file conflicts with --tofu-plan-file")
	}

	planFile, planProvisioner := selectPlanFile(c.parsedArgs.Run.TerraformPlanFile, c.parsedArgs.Run.TofuPlanFile)

	if planFile == "" && c.parsedArgs.Run.SyncPreview {
		fatal("--sync-preview requires --terraform-plan-file or -tofu-plan-file")
	}

	cloudSyncEnabled := c.parsedArgs.Run.SyncDeployment || c.parsedArgs.Run.SyncDriftStatus || c.parsedArgs.Run.SyncPreview

	if c.parsedArgs.Run.TerraformPlanFile != "" && !cloudSyncEnabled {
		fatal("--terraform-plan-file requires flags --sync-deployment or --sync-drift-status or --sync-preview")
	} else if c.parsedArgs.Run.TofuPlanFile != "" && !cloudSyncEnabled {
		fatal("--tofu-plan-file requires flags --sync-deployment or --sync-drift-status or --sync-preview")
	}

	c.checkTargetsConfiguration(c.parsedArgs.Run.Target, c.parsedArgs.Run.FromTarget, func(isTargetSet bool) {
		isStatusSet := c.parsedArgs.Run.Status != ""
		isUsingCloudFeat := cloudSyncEnabled || isStatusSet

		if isTargetSet && !isUsingCloudFeat {
			fatal("--target must be used together with --sync-deployment, --sync-drift-status, --sync-preview, or --status")
		} else if !isTargetSet && isUsingCloudFeat {
			fatal("--sync-*/--status flags require --target when terramate.config.cloud.targets.enabled is true")
		}
	})

	if c.parsedArgs.Run.FromTarget != "" && !cloudSyncEnabled {
		fatal("--from-target must be used together with --sync-deployment, --sync-drift-status, or --sync-preview")
	}

	if cloudSyncEnabled {
		if !c.prj.isRepo {
			fatal("cloud features requires a git repository")
		}
		c.ensureAllStackHaveIDs(stacks)
		c.detectCloudMetadata()
	}

	isCICD := os.Getenv("GITHUB_ACTIONS") != "" || os.Getenv("GITLAB_CI") != "" || os.Getenv("BITBUCKET_BUILD_NUMBER") != ""
	if c.parsedArgs.Run.SyncPreview && !isCICD {
		printer.Stderr.Warn(cloudSyncPreviewCICDWarning)
		c.disableCloudFeatures(errors.E(cloudSyncPreviewCICDWarning))
	}

	var runs []stackRun
	var err error
	for _, st := range stacks {
		run := stackRun{
			SyncTaskIndex: -1,
			Stack:         st.Stack,
			Tasks: []stackRunTask{
				{
					Cmd:                  c.parsedArgs.Run.Command,
					CloudTarget:          c.parsedArgs.Run.Target,
					CloudFromTarget:      c.parsedArgs.Run.FromTarget,
					CloudSyncDeployment:  c.parsedArgs.Run.SyncDeployment,
					CloudSyncDriftStatus: c.parsedArgs.Run.SyncDriftStatus,
					CloudSyncPreview:     c.parsedArgs.Run.SyncPreview,
					CloudPlanFile:        planFile,
					CloudPlanProvisioner: planProvisioner,
					CloudSyncLayer:       c.parsedArgs.Run.Layer,
					UseTerragrunt:        c.parsedArgs.Run.Terragrunt,
					EnableSharing:        c.parsedArgs.Run.EnableSharing,
					MockOnFail:           c.parsedArgs.Run.MockOnFail,
				},
			},
		}
		if c.parsedArgs.Run.Eval {
			run.Tasks[0].Cmd, err = c.evalRunArgs(run.Stack, run.Tasks[0].Cmd)
			if err != nil {
				fatalWithDetailf(err, "unable to evaluate command")
			}
		}
		runs = append(runs, run)
	}

	if c.parsedArgs.Run.SyncDeployment {
		// This will just select all runs, since the CloudSyncDeployment was set just above.
		// Still, it's convenient to re-use this function here.
		deployRuns := selectCloudStackTasks(runs, isDeploymentTask)
		c.createCloudDeployment(deployRuns)
	}

	if c.parsedArgs.Run.SyncPreview && c.cloudEnabled() {
		// See comment above.
		previewRuns := selectCloudStackTasks(runs, isPreviewTask)
		for metaID, previewID := range c.createCloudPreview(previewRuns, c.parsedArgs.Run.Target, c.parsedArgs.Run.FromTarget) {
			c.cloud.run.setMeta2PreviewID(metaID, previewID)
		}
	}

	err = c.runAll(runs, runAllOptions{
		Quiet:           c.parsedArgs.Quiet,
		DryRun:          c.parsedArgs.Run.DryRun,
		Reverse:         c.parsedArgs.Run.Reverse,
		ScriptRun:       false,
		ContinueOnError: c.parsedArgs.Run.ContinueOnError,
		Parallel:        c.parsedArgs.Run.Parallel,
	})
	if err != nil {
		fatalWithDetailf(err, "one or more commands failed")
	}
}

func (e *Engine) gitFileSafeguards(shouldAbort bool) {
	if e.parsedArgs.Run.DryRun {
		return
	}

	debugFiles(e.prj.git.repoChecks.UntrackedFiles, "untracked file")
	debugFiles(e.prj.git.repoChecks.UncommittedFiles, "uncommitted file")

	if e.checkGitUntracked() && len(e.prj.git.repoChecks.UntrackedFiles) > 0 {
		const msg = "repository has untracked files"
		if shouldAbort {
			fatal(msg)
		} else {
			log.Warn().Msg(msg)
		}
	}

	if e.checkGitUncommited() && len(e.prj.git.repoChecks.UncommittedFiles) > 0 {
		const msg = "repository has uncommitted files"
		if shouldAbort {
			fatal(msg)
		} else {
			log.Warn().Msg(msg)
		}
	}
}

// stackRunTask declares a stack run context.

func selectPlanFile(terraformPlan, tofuPlan string) (planfile, provisioner string) {
	if tofuPlan != "" {
		planfile = tofuPlan
		provisioner = ProvisionerOpenTofu
	} else if terraformPlan != "" {
		planfile = terraformPlan
		provisioner = ProvisionerTerraform
	}
	return
}

func (c *cli) runOnStacks() {
	c.gitSafeguardDefaultBranchIsReachable()

	if len(c.parsedArgs.Run.Command) == 0 {
		fatal("run expects a cmd")
	}

	c.checkOutdatedGeneratedCode()
	c.checkCloudSync()

	var stacks config.List[*config.SortableStack]
	if c.parsedArgs.Run.NoRecursive {
		st, found, err := config.TryLoadStack(c.cfg(), prj.PrjAbsPath(c.rootdir(), c.wd()))
		if err != nil {
			fatalWithDetailf(err, "loading stack in current directory")
		}

		if !found {
			fatal("--no-recursive provided but no stack found in the current directory")
		}

		stacks = append(stacks, st.Sortable())
	} else {
		var err error
		stackFilter := parseStatusFilter(c.parsedArgs.Run.Status)
		deploymentFilter := parseDeploymentStatusFilter(c.parsedArgs.Run.DeploymentStatus)
		driftFilter := parseDriftStatusFilter(c.parsedArgs.Run.DriftStatus)
		stacks, err = c.computeSelectedStacks(true, c.parsedArgs.Run.outputsSharingFlags, c.parsedArgs.Run.Target, cloud.StatusFilters{
			StackStatus:      stackFilter,
			DeploymentStatus: deploymentFilter,
			DriftStatus:      driftFilter,
		})
		if err != nil {
			fatal(err)
		}
	}

	if c.parsedArgs.Run.SyncDeployment && c.parsedArgs.Run.SyncDriftStatus {
		fatal("--sync-deployment conflicts with --sync-drift-status")
	}

	if c.parsedArgs.Run.SyncPreview && (c.parsedArgs.Run.SyncDeployment || c.parsedArgs.Run.SyncDriftStatus) {
		fatal("cannot use --sync-preview with --sync-deployment or --sync-drift-status")
	}

	if c.parsedArgs.Run.TerraformPlanFile != "" && c.parsedArgs.Run.TofuPlanFile != "" {
		fatal("--terraform-plan-file conflicts with --tofu-plan-file")
	}

	planFile, planProvisioner := selectPlanFile(c.parsedArgs.Run.TerraformPlanFile, c.parsedArgs.Run.TofuPlanFile)

	if planFile == "" && c.parsedArgs.Run.SyncPreview {
		fatal("--sync-preview requires --terraform-plan-file or -tofu-plan-file")
	}

	cloudSyncEnabled := c.parsedArgs.Run.SyncDeployment || c.parsedArgs.Run.SyncDriftStatus || c.parsedArgs.Run.SyncPreview

	if c.parsedArgs.Run.TerraformPlanFile != "" && !cloudSyncEnabled {
		fatal("--terraform-plan-file requires flags --sync-deployment or --sync-drift-status or --sync-preview")
	} else if c.parsedArgs.Run.TofuPlanFile != "" && !cloudSyncEnabled {
		fatal("--tofu-plan-file requires flags --sync-deployment or --sync-drift-status or --sync-preview")
	}

	c.checkTargetsConfiguration(c.parsedArgs.Run.Target, c.parsedArgs.Run.FromTarget, func(isTargetSet bool) {
		isStatusSet := c.parsedArgs.Run.Status != ""
		isUsingCloudFeat := cloudSyncEnabled || isStatusSet

		if isTargetSet && !isUsingCloudFeat {
			fatal("--target must be used together with --sync-deployment, --sync-drift-status, --sync-preview, or --status")
		} else if !isTargetSet && isUsingCloudFeat {
			fatal("--sync-*/--status flags require --target when terramate.config.cloud.targets.enabled is true")
		}
	})

	if c.parsedArgs.Run.FromTarget != "" && !cloudSyncEnabled {
		fatal("--from-target must be used together with --sync-deployment, --sync-drift-status, or --sync-preview")
	}

	if cloudSyncEnabled {
		if !c.prj.isRepo {
			fatal("cloud features requires a git repository")
		}
		c.ensureAllStackHaveIDs(stacks)
		c.detectCloudMetadata()
	}

	isCICD := os.Getenv("GITHUB_ACTIONS") != "" || os.Getenv("GITLAB_CI") != "" || os.Getenv("BITBUCKET_BUILD_NUMBER") != ""
	if c.parsedArgs.Run.SyncPreview && !isCICD {
		printer.Stderr.Warn(cloudSyncPreviewCICDWarning)
		c.disableCloudFeatures(errors.E(cloudSyncPreviewCICDWarning))
	}

	var runs []stackRun
	var err error
	for _, st := range stacks {
		run := stackRun{
			SyncTaskIndex: -1,
			Stack:         st.Stack,
			Tasks: []stackRunTask{
				{
					Cmd:                  c.parsedArgs.Run.Command,
					CloudTarget:          c.parsedArgs.Run.Target,
					CloudFromTarget:      c.parsedArgs.Run.FromTarget,
					CloudSyncDeployment:  c.parsedArgs.Run.SyncDeployment,
					CloudSyncDriftStatus: c.parsedArgs.Run.SyncDriftStatus,
					CloudSyncPreview:     c.parsedArgs.Run.SyncPreview,
					CloudPlanFile:        planFile,
					CloudPlanProvisioner: planProvisioner,
					CloudSyncLayer:       c.parsedArgs.Run.Layer,
					UseTerragrunt:        c.parsedArgs.Run.Terragrunt,
					EnableSharing:        c.parsedArgs.Run.EnableSharing,
					MockOnFail:           c.parsedArgs.Run.MockOnFail,
				},
			},
		}
		if c.parsedArgs.Run.Eval {
			run.Tasks[0].Cmd, err = c.evalRunArgs(run.Stack, run.Tasks[0].Cmd)
			if err != nil {
				fatalWithDetailf(err, "unable to evaluate command")
			}
		}
		runs = append(runs, run)
	}

	if c.parsedArgs.Run.SyncDeployment {
		// This will just select all runs, since the CloudSyncDeployment was set just above.
		// Still, it's convenient to re-use this function here.
		deployRuns := selectCloudStackTasks(runs, isDeploymentTask)
		c.createCloudDeployment(deployRuns)
	}

	if c.parsedArgs.Run.SyncPreview && c.cloudEnabled() {
		// See comment above.
		previewRuns := selectCloudStackTasks(runs, isPreviewTask)
		for metaID, previewID := range c.createCloudPreview(previewRuns, c.parsedArgs.Run.Target, c.parsedArgs.Run.FromTarget) {
			c.cloud.run.setMeta2PreviewID(metaID, previewID)
		}
	}

	err = c.runAll(runs, runAllOptions{
		Quiet:           c.parsedArgs.Quiet,
		DryRun:          c.parsedArgs.Run.DryRun,
		Reverse:         c.parsedArgs.Run.Reverse,
		ScriptRun:       false,
		ContinueOnError: c.parsedArgs.Run.ContinueOnError,
		Parallel:        c.parsedArgs.Run.Parallel,
	})
	if err != nil {
		fatalWithDetailf(err, "one or more commands failed")
	}
}

// runAllOptions define named flags for runAll
type runAllOptions struct {
	Quiet           bool
	DryRun          bool
	Reverse         bool
	ScriptRun       bool
	ContinueOnError bool
	Parallel        int
}

func (c *cli) createCloudPreview(runs []stackCloudRun, target, fromTarget string) map[string]string {
	previewRuns := make([]cloud.RunContext, len(runs))
	for i, run := range runs {
		previewRuns[i] = cloud.RunContext{
			StackID: run.Stack.ID,
			Cmd:     run.Task.Cmd,
		}
	}

	affectedStacksMap := map[string]cloud.Stack{}
	for _, st := range c.getAffectedStacks() {
		affectedStacksMap[st.Stack.ID] = cloud.Stack{
			Path:            st.Stack.Dir.String(),
			MetaID:          strings.ToLower(st.Stack.ID),
			MetaName:        st.Stack.Name,
			MetaDescription: st.Stack.Description,
			MetaTags:        st.Stack.Tags,
			Repository:      c.prj.prettyRepo(),
			Target:          target,
			FromTarget:      fromTarget,
			DefaultBranch:   c.prj.gitcfg().DefaultBranch,
		}
	}

	if c.cloud.run.reviewRequest == nil || c.cloud.run.rrEvent.pushedAt == nil {
		printer.Stderr.WarnWithDetails(
			"unable to create preview: missing review request information",
			errors.E("--sync-preview can only be used when GITHUB_TOKEN or GITLAB_TOKEN is exported and Terramate runs in a CI/CD environment triggered by a Pull/Merge Request event"),
		)
		c.disableCloudFeatures(cloudError())
		return map[string]string{}
	}

	c.cloud.run.reviewRequest.PushedAt = c.cloud.run.rrEvent.pushedAt

	// preview always requires a commit_sha, so if the API failed to provide it, we should give the HEAD commit.
	if c.cloud.run.rrEvent.commitSHA == "" {
		c.cloud.run.rrEvent.commitSHA = c.prj.headCommit()
	}

	technology := "other"
	technologyLayer := "default"
	for _, run := range runs {
		if run.Task.CloudPlanFile != "" {
			technology = run.Task.CloudPlanProvisioner
		}
		if layer := run.Task.CloudSyncLayer; layer != "" {
			technologyLayer = stdfmt.Sprintf("custom:%s", layer)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultCloudTimeout)
	defer cancel()
	createdPreview, err := c.cloud.client.CreatePreview(
		ctx,
		cloud.CreatePreviewOpts{
			Runs:            previewRuns,
			AffectedStacks:  affectedStacksMap,
			OrgUUID:         c.cloud.run.orgUUID,
			PushedAt:        *c.cloud.run.rrEvent.pushedAt,
			CommitSHA:       c.cloud.run.rrEvent.commitSHA,
			Technology:      technology,
			TechnologyLayer: technologyLayer,
			ReviewRequest:   c.cloud.run.reviewRequest,
			Metadata:        c.cloud.run.metadata,
		},
	)
	if err != nil {
		printer.Stderr.WarnWithDetails("unable to create preview", err)
		c.disableCloudFeatures(cloudError())
		return map[string]string{}
	}

	printer.Stderr.Success(stdfmt.Sprintf("Preview created (id: %s)", createdPreview.ID))

	if c.parsedArgs.Run.DebugPreviewURL != "" {
		c.writePreviewURL()
	}

	return createdPreview.StackPreviewsByMetaID
}

func (c *cli) writePreviewURL() {
	rrNumber := 0
	if c.cloud.run.metadata != nil && c.cloud.run.metadata.GithubPullRequestNumber != 0 {
		ctx, cancel := context.WithTimeout(context.Background(), defaultCloudTimeout)
		defer cancel()
		reviews, err := c.cloud.client.ListReviewRequests(ctx, c.cloud.run.orgUUID)
		if err != nil {
			printer.Stderr.Warn(stdfmt.Sprintf("unable to list review requests: %v", err))
			return
		}
		for _, review := range reviews {
			if review.Number == c.cloud.run.metadata.GithubPullRequestNumber &&
				review.CommitSHA == c.prj.headCommit() {
				rrNumber = int(review.ID)
			}
		}
	}

	cloudURL := cloud.HTMLURL(c.cloud.client.Region)
	if c.cloud.client.BaseURL == "https://api.stg.terramate.io" {
		cloudURL = "https://cloud.stg.terramate.io"
	}

	var url = stdfmt.Sprintf("%s/o/%s/review-requests\n", cloudURL, c.cloud.run.orgName)
	if rrNumber != 0 {
		url = stdfmt.Sprintf("%s/o/%s/review-requests/%d\n",
			cloudURL,
			c.cloud.run.orgName,
			rrNumber)
	}

	err := os.WriteFile(c.parsedArgs.Run.DebugPreviewURL, []byte(url), 0644)
	if err != nil {
		printer.Stderr.Warn(stdfmt.Sprintf("unable to write preview URL to file: %v", err))
	}
}

// getAffectedStacks returns the list of stacks affected by the current command.
// c.affectedStacks is expected to be already set, if not it will be computed
// and cached.
func (c *cli) getAffectedStacks() []stack.Entry {
	if c.affectedStacks != nil {
		return c.affectedStacks
	}

	mgr := c.stackManager()

	var report *stack.Report
	var err error
	if c.parsedArgs.Changed {
		report, err = mgr.ListChanged(stack.ChangeConfig{
			BaseRef:            c.baseRef(),
			UntrackedChanges:   c.changeDetection.untracked,
			UncommittedChanges: c.changeDetection.uncommitted,
		})
		if err != nil {
			fatalWithDetailf(err, "listing changed stacks")
		}

	} else {
		report, err = mgr.List(true)
		if err != nil {
			fatalWithDetailf(err, "listing stacks")
		}
	}

	c.affectedStacks = report.Stacks
	return c.affectedStacks
}

const targetIDRegexPattern = "^[a-z0-9][-_a-z0-9]*[a-z0-9]$"

var targetIDRegex = regexp.MustCompile(targetIDRegexPattern)

func (c *cli) checkTargetsConfiguration(targetArg, fromTargetArg string, cloudCheckFn func(bool)) {
	isTargetSet := targetArg != ""
	isFromTargetSet := fromTargetArg != ""
	isTargetsEnabled := c.cfg().HasExperiment("targets") && c.cfg().IsTargetsEnabled()

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
		cloudCheckFn(true)

	} else {
		if isTargetsEnabled {
			// Here we should check if any cloud parameter is enabled that would require target.
			// The error messages should be different per caller.
			cloudCheckFn(false)
		}
	}

	if isFromTargetSet && !isTargetSet {
		fatal("--from-target requires --target")
	}

	if isTargetSet && !targetIDRegex.MatchString(targetArg) {
		fatalf("--target value has invalid format, it must match %q", targetIDRegexPattern)
	}

	if isFromTargetSet && !targetIDRegex.MatchString(fromTargetArg) {
		fatalf("--from-target value has invalid format, it must match %q", targetIDRegexPattern)
	}

	c.cloud.run.target = targetArg
}

func (c *cli) setupSafeguards(run runSafeguardsCliSpec) {
	global := c.parsedArgs.deprecatedGlobalSafeguardsCliSpec

	// handle deprecated flags as --disable-safeguards
	if global.DeprecatedDisableCheckGitUncommitted {
		run.DisableSafeguards = append(run.DisableSafeguards, "git-uncommitted")
	}
	if global.DeprecatedDisableCheckGitUntracked {
		run.DisableSafeguards = append(run.DisableSafeguards, "git-untracked")
	}
	if run.DeprecatedDisableCheckGitRemote {
		run.DisableSafeguards = append(run.DisableSafeguards, "git-out-of-sync")
	}
	if run.DeprecatedDisableCheckGenCode {
		run.DisableSafeguards = append(run.DisableSafeguards, "outdated-code")
	}
	if run.DisableSafeguardsAll {
		run.DisableSafeguards = append(run.DisableSafeguards, "all")
	}

	if run.DisableSafeguards.Has(safeguard.All) && run.DisableSafeguards.Has(safeguard.None) {
		fatalWithDetailf(
			errors.E(clitest.ErrSafeguardKeywordValidation,
				`the safeguards keywords "all" and "none" are incompatible`),
			"Disabling safeguards",
		)
	}

	c.safeguards.DisableCheckGitUncommitted = run.DisableSafeguards.Has(safeguard.GitUncommitted, safeguard.All, safeguard.Git)
	c.safeguards.DisableCheckGitUntracked = run.DisableSafeguards.Has(safeguard.GitUntracked, safeguard.All, safeguard.Git)
	c.safeguards.DisableCheckGitRemote = run.DisableSafeguards.Has(safeguard.GitOutOfSync, safeguard.All, safeguard.Git)
	c.safeguards.DisableCheckGenerateOutdatedCheck = run.DisableSafeguards.Has(safeguard.Outdated, safeguard.All)
	if run.DisableSafeguards.Has("none") {
		c.safeguards = Safeguards{}
		c.safeguards.reEnabled = true
	}
}
