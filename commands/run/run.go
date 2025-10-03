// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package run provides the run command.
package run

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/api/preview"
	"github.com/terramate-io/terramate/cloud/api/resources"
	"github.com/terramate-io/terramate/cloud/api/status"
	"github.com/terramate-io/terramate/cloudsync"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/project"
	"github.com/zclconf/go-cty/cty"
)

const (
	// ErrConflictOptions tells if the error is related to conflicting options in the command spec.
	ErrConflictOptions errors.Kind = "conflicting arguments"
	// ErrCurrentHeadIsOutOfDate indicates the local HEAD revision is outdated.
	ErrCurrentHeadIsOutOfDate errors.Kind = "current HEAD is out-of-date with the remote base branch"
	// ErrOutdatedGenCodeDetected indicates outdated generated code detected.
	ErrOutdatedGenCodeDetected errors.Kind = "outdated generated code detected"

	cloudSyncPreviewCICDWarning = "--sync-preview is only supported in GitHub Actions workflows, Gitlab CICD pipelines or Bitbucket Cloud Pipelines"
)

// Spec is the command specification for the run command.
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

	SyncDeployment    bool
	SyncDriftStatus   bool
	SyncPreview       bool
	DebugPreviewURL   string
	TechnologyLayer   preview.Layer
	TerraformPlanFile string
	PlanRenderTimeout time.Duration
	TofuPlanFile      string
	Terragrunt        bool
	EnableSharing     bool
	MockOnFail        bool
	EvalCmd           bool

	GitFilter     engine.GitFilter
	StatusFilters StatusFilters
	Target        string
	FromTarget    string
	Tags          []string
	NoTags        []string

	engine.OutputsSharingOptions

	Safeguards Safeguards

	state cloudsync.CloudRunState
}

// StatusFilters holds the status filters for the run command.
type StatusFilters struct {
	StackStatus      string
	DeploymentStatus string
	DriftStatus      string
}

// Safeguards holds the safeguard options for the run command.
type Safeguards struct {
	DisableCheckGitUntracked          bool
	DisableCheckGitUncommitted        bool
	DisableCheckGitRemote             bool
	DisableCheckGenerateOutdatedCheck bool

	ReEnabled bool
}

// Name returns the name of the command.
func (s *Spec) Name() string { return "run" }

// Exec executes the run command.
func (s *Spec) Exec(ctx context.Context) error {
	if len(s.Command) == 0 {
		return errors.E("run expects a command")
	}

	err := CheckOutdatedGeneratedCode(ctx, s.Engine, s.Safeguards, s.WorkingDir)
	if err != nil {
		return err
	}
	err = s.checkCloudSync()
	if err != nil {
		return err
	}

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
		stacks, err = s.Engine.AddOutputDependencies(s.OutputsSharingOptions, stacks, s.Target)
		if err != nil {
			return err
		}
	} else {
		cloudFilters, err := status.ParseFilters(
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
		stacks, err = s.Engine.ComputeSelectedStacks(s.GitFilter, tags, s.OutputsSharingOptions, s.Target, cloudFilters)
		if err != nil {
			return err
		}

		if !s.DryRun {
			err = GitFileSafeguards(s.Engine, true, s.Safeguards)
			if err != nil {
				return err
			}
		}
	}

	err = GitSafeguardDefaultBranchIsReachable(s.Engine, s.Safeguards)
	if err != nil {
		return err
	}

	if s.SyncDeployment && s.SyncDriftStatus {
		return errors.E(ErrConflictOptions, "--sync-deployment conflicts with --sync-drift-status")
	}

	if s.SyncPreview && (s.SyncDeployment || s.SyncDriftStatus) {
		return errors.E(ErrConflictOptions, "cannot use --sync-preview with --sync-deployment or --sync-drift-status")
	}

	if s.TerraformPlanFile != "" && s.TofuPlanFile != "" {
		return errors.E(ErrConflictOptions, "--terraform-plan-file conflicts with --tofu-plan-file")
	}

	planFile, planProvisioner := SelectPlanFile(s.TerraformPlanFile, s.TofuPlanFile)

	if planFile == "" && s.SyncPreview {
		return errors.E(ErrConflictOptions, "--sync-preview requires --terraform-plan-file or -tofu-plan-file")
	}

	cloudSyncEnabled := s.SyncDeployment || s.SyncDriftStatus || s.SyncPreview

	if s.TerraformPlanFile != "" && !cloudSyncEnabled {
		return errors.E(ErrConflictOptions, "--terraform-plan-file requires flags --sync-deployment or --sync-drift-status or --sync-preview")
	} else if s.TofuPlanFile != "" && !cloudSyncEnabled {
		return errors.E(ErrConflictOptions, "--tofu-plan-file requires flags --sync-deployment or --sync-drift-status or --sync-preview")
	}

	err = s.Engine.CheckTargetsConfiguration(s.Target, s.FromTarget, func(isTargetSet bool) error {
		isStatusSet := s.StatusFilters.StackStatus != ""
		isUsingCloudFeat := cloudSyncEnabled || isStatusSet

		if isTargetSet && !isUsingCloudFeat {
			return errors.E(ErrConflictOptions, "--target must be used together with --sync-deployment, --sync-drift-status, --sync-preview, or --status")
		} else if !isTargetSet && isUsingCloudFeat {
			return errors.E(ErrConflictOptions, "--sync-*/--status flags require --target when terramate.config.cloud.targets.enabled is true")
		}
		return nil
	})

	if err != nil {
		return err
	}

	if s.FromTarget != "" && !cloudSyncEnabled {
		return errors.E(ErrConflictOptions, "--from-target must be used together with --sync-deployment, --sync-drift-status, or --sync-preview")
	}

	if cloudSyncEnabled {
		if !s.Engine.Project().IsRepo() {
			return errors.E("cloud features requires a git repository")
		}
		err = s.Engine.EnsureAllStackHaveIDs(stacks)
		if err != nil {
			return err
		}

		cloudsync.DetectCloudMetadata(s.Engine, &s.state)
	}

	isCICD := os.Getenv("GITHUB_ACTIONS") != "" || os.Getenv("GITLAB_CI") != "" || os.Getenv("BITBUCKET_BUILD_NUMBER") != ""
	if s.SyncPreview && !isCICD {
		printer.Stderr.Warn(cloudSyncPreviewCICDWarning)
		s.Engine.DisableCloudFeatures(errors.E(cloudSyncPreviewCICDWarning))
	}

	var runs []engine.StackRun
	for _, st := range stacks {
		run := engine.StackRun{
			SyncTaskIndex: -1,
			Stack:         st.Stack,
			Tasks: []engine.StackRunTask{
				{
					Cmd:                    s.Command,
					CloudTarget:            s.Target,
					CloudFromTarget:        s.FromTarget,
					CloudSyncDeployment:    s.SyncDeployment,
					CloudSyncDriftStatus:   s.SyncDriftStatus,
					CloudSyncPreview:       s.SyncPreview,
					CloudPlanFile:          planFile,
					CloudPlanProvisioner:   planProvisioner,
					CloudPlanRenderTimeout: s.PlanRenderTimeout,
					CloudSyncLayer:         s.TechnologyLayer,
					UseTerragrunt:          s.Terragrunt,
					EnableSharing:          s.EnableSharing,
					MockOnFail:             s.MockOnFail,
				},
			},
		}
		if s.EvalCmd {
			run.Tasks[0].Cmd, err = s.evalRunArgs(run.Stack, s.Target, run.Tasks[0].Cmd)
			if err != nil {
				return errors.D("%s", "unable to evaluate command").WithError(err)
			}
		}
		runs = append(runs, run)
	}

	if s.SyncDeployment {
		// This will just select all runs, since the CloudSyncDeployment was set just above.
		// Still, it's convenient to re-use this function here.
		deployRuns := engine.SelectCloudStackTasks(runs, engine.IsDeploymentTask)
		err := cloudsync.CreateCloudDeployment(s.Engine, s.WorkingDir, deployRuns, &s.state)
		if err != nil {
			return err
		}
	}

	if s.SyncPreview && s.cloudEnabled() {
		// See comment above.
		previewRuns := engine.SelectCloudStackTasks(runs, engine.IsPreviewTask)
		for metaID, previewID := range cloudsync.CreateCloudPreview(s.Engine, s.GitFilter, previewRuns, s.Target, s.FromTarget, &s.state) {
			s.state.SetMeta2PreviewID(metaID, previewID)
		}

		if s.DebugPreviewURL != "" {
			s.writePreviewURL()
		}
	}

	err = s.Engine.RunAll(runs, engine.RunAllOptions{
		Quiet:           s.Quiet,
		DryRun:          s.DryRun,
		Reverse:         s.Reverse,
		ScriptRun:       false,
		ContinueOnError: s.ContinueOnError,
		Parallel:        s.Parallel,
		Stdout:          s.Stdout,
		Stderr:          s.Stderr,
		Stdin:           s.Stdin,
		Hooks: &engine.Hooks{
			Before: func(e *engine.Engine, run engine.StackCloudRun) {
				cloudsync.BeforeRun(e, run, &s.state)
			},
			After: func(e *engine.Engine, run engine.StackCloudRun, res engine.RunResult, err error) {
				cloudsync.AfterRun(e, run, &s.state, res, err)
			},
			LogSyncCondition: func(task engine.StackRunTask, _ engine.StackRun) bool {
				return task.CloudSyncDeployment || task.CloudSyncPreview
			},
			LogSyncer: func(logger *zerolog.Logger, e *engine.Engine, run engine.StackRun, logs resources.CommandLogs) {
				cloudsync.Logs(logger, e, run, &s.state, logs)
			},
		},
	})
	if err != nil {
		return errors.D("%s", "one or more commands failed").WithError(err)
	}
	return nil
}

func (s *Spec) cloudEnabled() bool { return s.Engine.IsCloudEnabled() }

// SelectPlanFile returns the plan file and provisioner to use based on the provided flags.
func SelectPlanFile(terraformPlan, tofuPlan string) (planfile, provisioner string) {
	if tofuPlan != "" {
		planfile = tofuPlan
		provisioner = cloudsync.ProvisionerOpenTofu
	} else if terraformPlan != "" {
		planfile = terraformPlan
		provisioner = cloudsync.ProvisionerTerraform
	}
	return
}

func (s *Spec) evalRunArgs(st *config.Stack, target string, cmd []string) ([]string, error) {
	ctx, err := s.Engine.SetupEvalContext(st.HostDir(s.Engine.Config()), st, target, map[string]string{})
	if err != nil {
		return nil, err
	}
	var newargs []string
	for _, arg := range cmd {
		exprStr := `"` + arg + `"`
		expr, err := ast.ParseExpression(exprStr, "<cmd arg>")
		if err != nil {
			return nil, errors.E(err, "parsing %s", exprStr)
		}
		val, err := ctx.Eval(expr)
		if err != nil {
			return nil, errors.E(err, "eval %s", exprStr)
		}
		if !val.Type().Equals(cty.String) {
			return nil, errors.E("cmd line evaluates to type %s but only string is permitted", val.Type().FriendlyName())
		}

		newargs = append(newargs, val.AsString())
	}
	return newargs, nil
}

func (s *Spec) writePreviewURL() {
	client := s.Engine.CloudClient()
	rrNumber := 0
	if s.state.Metadata != nil && s.state.Metadata.GithubPullRequestNumber != 0 {
		ctx, cancel := context.WithTimeout(context.Background(), cloud.DefaultTimeout)
		defer cancel()
		reviews, err := client.ListReviewRequests(ctx, s.Engine.CloudState().Org.UUID)
		if err != nil {
			printer.Stderr.Warn(fmt.Sprintf("unable to list review requests: %v", err))
			return
		}
		headCommit, err := s.Engine.Project().HeadCommit()
		if err != nil {
			printer.Stderr.Warn(fmt.Sprintf("unable to get head commit: %v", err))
			return
		}
		for _, review := range reviews {
			if review.Number == s.state.Metadata.GithubPullRequestNumber &&
				review.CommitSHA == headCommit {
				rrNumber = int(review.ID)
			}
		}
	}

	cloudURL := cloud.HTMLURL(client.Region())
	if client.BaseURL() == "https://api.stg.terramate.io" {
		cloudURL = "https://cloud.stg.terramate.io"
	}

	var url = fmt.Sprintf("%s/o/%s/review-requests\n", cloudURL, s.Engine.CloudState().Org.Name)
	if rrNumber != 0 {
		url = fmt.Sprintf("%s/o/%s/review-requests/%d\n",
			cloudURL,
			s.Engine.CloudState().Org.Name,
			rrNumber)
	}

	err := os.WriteFile(s.DebugPreviewURL, []byte(url), 0644)
	if err != nil {
		printer.Stderr.Warn(fmt.Sprintf("unable to write preview URL to file: %v", err))
	}
}
