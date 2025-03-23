// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"context"

	stdfmt "fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/api/resources"
	"github.com/terramate-io/terramate/cloud/api/status"
	"github.com/terramate-io/terramate/cloudsync"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/config/filter"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/printer"
	prj "github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/safeguard"
)

const (
	// ErrRunFailed represents the error when the execution fails, whatever the reason.
	ErrRunFailed errors.Kind = "execution failed"

	// ErrRunCanceled represents the error when the execution was canceled.
	ErrRunCanceled errors.Kind = "execution canceled"

	// ErrRunCommandNotExecuted represents the error when the command was not executed for whatever reason.
	ErrRunCommandNotExecuted errors.Kind = "command not found"

	cloudSyncPreviewCICDWarning = "--sync-preview is only supported in GitHub Actions workflows, Gitlab CICD pipelines or Bitbucket Cloud Pipelines"
)

func selectPlanFile(terraformPlan, tofuPlan string) (planfile, provisioner string) {
	if tofuPlan != "" {
		planfile = tofuPlan
		provisioner = cloudsync.ProvisionerOpenTofu
	} else if terraformPlan != "" {
		planfile = terraformPlan
		provisioner = cloudsync.ProvisionerTerraform
	}
	return
}

func (c *cli) runOnStacks() {
	if len(c.parsedArgs.Run.Command) == 0 {
		fatal("run expects a cmd")
	}

	c.checkOutdatedGeneratedCode()

	var cloudState cloudsync.CloudRunState
	c.checkCloudSync(&cloudState)

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
		gitfilter, err := engine.NewGitFilter(c.parsedArgs.Changed, c.parsedArgs.GitChangeBase, c.parsedArgs.Run.EnableChangeDetection, c.parsedArgs.Run.DisableChangeDetection)
		if err != nil {
			fatal(err)
		}
		stackFilters, err := status.ParseFilters(c.parsedArgs.Run.Status, c.parsedArgs.Run.DeploymentStatus, c.parsedArgs.Run.DriftStatus)
		if err != nil {
			fatal(err)
		}
		tagFilters, err := filter.ParseTags(c.parsedArgs.Tags, c.parsedArgs.NoTags)
		if err != nil {
			fatal(err)
		}
		stacks, err = c.engine.ComputeSelectedStacks(
			gitfilter,
			tagFilters,
			engine.OutputsSharingOptions{
				IncludeOutputDependencies: c.parsedArgs.Run.IncludeOutputDependencies,
				OnlyOutputDependencies:    c.parsedArgs.Run.OnlyOutputDependencies,
			},
			c.parsedArgs.Run.Target,
			stackFilters,
		)
		if err != nil {
			fatal(err)
		}

		if !c.parsedArgs.Run.DryRun {
			err := gitFileSafeguards(c.engine, true, c.safeguards)
			if err != nil {
				fatal(err)
			}
		}
	}

	c.gitSafeguardDefaultBranchIsReachable()

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

	err := c.engine.CheckTargetsConfiguration(c.parsedArgs.Run.Target, c.parsedArgs.Run.FromTarget, func(isTargetSet bool) error {
		isStatusSet := c.parsedArgs.Run.Status != ""
		isUsingCloudFeat := cloudSyncEnabled || isStatusSet

		if isTargetSet && !isUsingCloudFeat {
			return errors.E("--target must be used together with --sync-deployment, --sync-drift-status, --sync-preview, or --status")
		} else if !isTargetSet && isUsingCloudFeat {
			return errors.E("--sync-*/--status flags require --target when terramate.config.cloud.targets.enabled is true")
		}
		return nil
	})
	if c.engine.HandleCloudCriticalError(err) != nil {
		fatal(err)
	}

	if c.parsedArgs.Run.FromTarget != "" && !cloudSyncEnabled {
		fatal("--from-target must be used together with --sync-deployment, --sync-drift-status, or --sync-preview")
	}

	if cloudSyncEnabled {
		if !c.project().IsRepo() {
			fatal("cloud features requires a git repository")
		}
		c.ensureAllStackHaveIDs(stacks)

		cloudsync.DetectCloudMetadata(c.engine, &cloudState)
	}

	isCICD := os.Getenv("GITHUB_ACTIONS") != "" || os.Getenv("GITLAB_CI") != "" || os.Getenv("BITBUCKET_BUILD_NUMBER") != ""
	if c.parsedArgs.Run.SyncPreview && !isCICD {
		printer.Stderr.Warn(cloudSyncPreviewCICDWarning)
		c.engine.DisableCloudFeatures(errors.E(cloudSyncPreviewCICDWarning))
	}

	var runs []engine.StackRun
	for _, st := range stacks {
		run := engine.StackRun{
			SyncTaskIndex: -1,
			Stack:         st.Stack,
			Tasks: []engine.StackRunTask{
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

	if c.parsedArgs.Run.SyncDeployment && c.engine.IsCloudEnabled() {
		// This will just select all runs, since the CloudSyncDeployment was set just above.
		// Still, it's convenient to re-use this function here.
		deployRuns := engine.SelectCloudStackTasks(runs, engine.IsDeploymentTask)

		err := cloudsync.CreateCloudDeployment(c.engine, c.wd(), deployRuns, &cloudState)
		if err != nil {
			fatalWithDetailf(err, "unable to create cloud deployment")
		}
	}

	if c.parsedArgs.Run.SyncPreview && c.engine.IsCloudEnabled() {
		// See comment above.
		previewRuns := engine.SelectCloudStackTasks(runs, engine.IsPreviewTask)
		gitfilter, err := engine.NewGitFilter(c.parsedArgs.Changed, c.parsedArgs.GitChangeBase, c.parsedArgs.Run.EnableChangeDetection, c.parsedArgs.Run.DisableChangeDetection)
		if err != nil {
			fatal(err)
		}

		for metaID, previewID := range cloudsync.CreateCloudPreview(c.engine, gitfilter, previewRuns, c.parsedArgs.Run.Target, c.parsedArgs.Run.FromTarget, &cloudState) {
			cloudState.SetMeta2PreviewID(metaID, previewID)
		}

		if c.parsedArgs.Run.DebugPreviewURL != "" {
			c.writePreviewURL(&cloudState)
		}
	}

	err = c.engine.RunAll(runs, engine.RunAllOptions{
		Quiet:           c.parsedArgs.Quiet,
		DryRun:          c.parsedArgs.Run.DryRun,
		Reverse:         c.parsedArgs.Run.Reverse,
		ScriptRun:       false,
		ContinueOnError: c.parsedArgs.Run.ContinueOnError,
		Parallel:        c.parsedArgs.Run.Parallel,
		Stdout:          c.stdout,
		Stderr:          c.stderr,
		Stdin:           c.stdin,
		Hooks: &engine.Hooks{
			Before: func(e *engine.Engine, run engine.StackCloudRun) {
				cloudsync.BeforeRun(e, run, &cloudState)
			},
			After: func(e *engine.Engine, run engine.StackCloudRun, res engine.RunResult, err error) {
				cloudsync.AfterRun(e, run, &cloudState, res, err)
			},
			LogSyncCondition: func(task engine.StackRunTask, _ engine.StackRun) bool {
				return task.CloudSyncDeployment || task.CloudSyncPreview
			},
			LogSyncer: func(logger *zerolog.Logger, e *engine.Engine, run engine.StackRun, logs resources.CommandLogs) {
				cloudsync.Logs(logger, e, run, &cloudState, logs)
			},
		},
	})
	if err != nil {
		fatalWithDetailf(err, "one or more commands failed")
	}
}

func (c *cli) writePreviewURL(state *cloudsync.CloudRunState) {
	headCommit, err := c.project().HeadCommit()
	if err != nil {
		printer.Stderr.Warn(stdfmt.Sprintf("unable to retrieve head commit: %v", err))
		return
	}

	client := c.engine.CloudClient()
	rrNumber := 0
	if state.Metadata != nil && state.Metadata.GithubPullRequestNumber != 0 {
		ctx, cancel := context.WithTimeout(context.Background(), cloud.DefaultTimeout)
		defer cancel()

		reviews, err := client.ListReviewRequests(ctx, c.engine.CloudState().Org.UUID)
		if err != nil {
			printer.Stderr.Warn(stdfmt.Sprintf("unable to list review requests: %v", err))
			return
		}
		for _, review := range reviews {
			if review.Number == state.Metadata.GithubPullRequestNumber &&
				review.CommitSHA == headCommit {
				rrNumber = int(review.ID)
			}
		}
	}

	cloudURL := cloud.HTMLURL(client.Region())
	if client.BaseURL() == "https://api.stg.terramate.io" {
		cloudURL = "https://cloud.stg.terramate.io"
	}

	orgName := c.engine.CloudState().Org.Name
	var url = stdfmt.Sprintf("%s/o/%s/review-requests\n", cloudURL, orgName)
	if rrNumber != 0 {
		url = stdfmt.Sprintf("%s/o/%s/review-requests/%d\n",
			cloudURL,
			orgName,
			rrNumber)
	}

	err = os.WriteFile(c.parsedArgs.Run.DebugPreviewURL, []byte(url), 0644)
	if err != nil {
		printer.Stderr.Warn(stdfmt.Sprintf("unable to write preview URL to file: %v", err))
	}
}

// gitFileSafeguards checks for untracked and uncommitted files in the repository.
func gitFileSafeguards(e *engine.Engine, shouldError bool, sf safeguards) error {
	repochecks := e.RepoChecks()
	debugFiles(repochecks.UntrackedFiles, "untracked file")
	debugFiles(repochecks.UncommittedFiles, "uncommitted file")

	if checkGitUntracked(e, sf) && len(repochecks.UntrackedFiles) > 0 {
		const msg = "repository has untracked files"
		if shouldError {
			return errors.E(msg)
		}
		log.Warn().Msg(msg)
	}

	if checkGitUncommited(e, sf) && len(repochecks.UncommittedFiles) > 0 {
		const msg = "repository has uncommitted files"
		if shouldError {
			return errors.E(msg)
		}
		log.Warn().Msg(msg)
	}
	return nil
}

func checkGitUntracked(e *engine.Engine, sf safeguards) bool {
	if !e.Project().IsGitFeaturesEnabled() || sf.DisableCheckGitUntracked {
		return false
	}

	if sf.reEnabled {
		return !sf.DisableCheckGitUntracked
	}

	cfg := e.RootNode()
	if cfg.Terramate == nil || cfg.Terramate.Config == nil {
		return true
	}
	return !cfg.Terramate.Config.HasSafeguardDisabled(safeguard.GitUntracked)
}

func checkGitUncommited(e *engine.Engine, sf safeguards) bool {
	if !e.Project().IsGitFeaturesEnabled() || sf.DisableCheckGitUncommitted {
		return false
	}

	if sf.reEnabled {
		return !sf.DisableCheckGitUncommitted
	}

	cfg := e.RootNode()
	if cfg.Terramate == nil || cfg.Terramate.Config == nil {
		return true
	}
	return !cfg.Terramate.Config.HasSafeguardDisabled(safeguard.GitUncommitted)
}
