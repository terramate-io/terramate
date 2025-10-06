// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package run provides the script run command.
package run

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/hashicorp/go-uuid"
	"github.com/rs/zerolog"
	"github.com/terramate-io/terramate/cloud/api/resources"
	"github.com/terramate-io/terramate/cloud/api/status"
	"github.com/terramate-io/terramate/cloudsync"

	runcmd "github.com/terramate-io/terramate/commands/run"
	"github.com/terramate-io/terramate/commands/script"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/globals"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/stdlib"
	"github.com/zclconf/go-cty/cty"

	tel "github.com/terramate-io/terramate/ui/tui/telemetry"
)

const (
	cloudFeatScriptSyncDeployment  = "Script option 'sync_deployment' is a Terramate Cloud feature to synchronize deployment details to Terramate Cloud."
	cloudFeatScriptSyncDriftStatus = "Script option 'sync_drift_status' is a Terramate Cloud feature to synchronize drift and health check results to Terramate Cloud."
	cloudFeatScriptSyncPreview     = "Script option 'sync_preview' is a Terramate Cloud feature to synchronize deployment previews to Terramate Cloud."
)

const cloudSyncPreviewCICDWarning = "--sync-preview is only supported in GitHub Actions workflows, Gitlab CICD pipelines or Bitbucket Cloud Pipelines"

// Spec represents the script run specification.
type Spec struct {
	WorkingDir string
	Engine     *engine.Engine
	Safeguards runcmd.Safeguards
	Printers   printer.Printers

	DryRun          bool
	Quiet           bool
	Reverse         bool
	Parallel        int
	ContinueOnError bool
	GitFilter       engine.GitFilter
	Target          string
	FromTarget      string
	NoRecursive     bool
	NoTags          []string
	Tags            []string
	engine.OutputsSharingOptions
	StatusFilters runcmd.StatusFilters

	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader

	Labels []string

	state cloudsync.CloudRunState
}

// Name returns the name of the script run command.
func (s *Spec) Name() string { return "script run" }

// Exec executes the script run command.
func (s *Spec) Exec(ctx context.Context) error {
	err := runcmd.CheckOutdatedGeneratedCode(ctx, s.Engine, s.Safeguards, s.WorkingDir)
	if err != nil {
		return err
	}

	err = s.Engine.CheckTargetsConfiguration(s.Target, s.FromTarget, func(isTargetSet bool) error {
		if !isTargetSet {
			// We don't check here if any script has any sync command options enabled.
			// We assume yes and so --target must be set.
			return errors.E("--target is required when terramate.config.cloud.targets.enabled is true")
		}
		return nil
	})

	if err != nil {
		return err
	}

	root := s.Engine.Config()

	var stacks config.List[*config.SortableStack]
	if s.NoRecursive {
		st, found, err := config.TryLoadStack(root, project.PrjAbsPath(root.HostDir(), s.WorkingDir))
		if err != nil {
			return errors.E(err, "failed to load stack in current directory")
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
			err = runcmd.GitFileSafeguards(s.Engine, true, s.Safeguards)
			if err != nil {
				return err
			}
		}
	}

	err = runcmd.GitSafeguardDefaultBranchIsReachable(s.Engine, s.Safeguards)
	if err != nil {
		return err
	}

	// search for the script and prepare a list of script/stack entries
	m := script.NewMatcher(s.Labels)
	m.Search(root, stacks)

	if len(m.Results) == 0 {
		return errors.E(color.RedString("script not found: ") + strings.Join(s.Labels, " "))
	}

	if s.DryRun {
		s.Printers.Stderr.Println("This is a dry run, commands will not be executed.")
	}

	var runs []engine.StackRun

	for scriptIdx, result := range m.Results {
		if len(result.Stacks) == 0 {
			continue
		}

		if !s.Quiet {
			s.Printers.Stderr.Println(fmt.Sprintf("Script %s at %s having %s job(s)",
				color.GreenString(fmt.Sprintf("%d", scriptIdx)),
				color.BlueString(result.ScriptCfg.Range.String()),
				color.BlueString(fmt.Sprintf("%d", len(result.ScriptCfg.Jobs))),
			))
		}

		for _, st := range result.Stacks {
			run := engine.StackRun{Stack: st.Stack}

			ectx, err := scriptEvalContext(root, st.Stack, s.Target)
			if err != nil {
				return errors.E(err, "failed to get context")
			}

			evalScript, err := config.EvalScript(ectx, *result.ScriptCfg)
			if err != nil {
				return errors.E(err, "failed to eval script")
			}

			for jobIdx, job := range evalScript.Jobs {
				for cmdIdx, cmd := range job.Commands() {
					task := engine.StackRunTask{
						Cmd:             cmd.Args,
						CloudTarget:     s.Target,
						CloudFromTarget: s.FromTarget,
						ScriptIdx:       scriptIdx,
						ScriptJobIdx:    jobIdx,
						ScriptCmdIdx:    cmdIdx,
					}

					if cmd.Options != nil {
						planFile, planProvisioner := runcmd.SelectPlanFile(
							cmd.Options.CloudTerraformPlanFile,
							cmd.Options.CloudTofuPlanFile)

						task.CloudSyncDeployment = cmd.Options.CloudSyncDeployment
						task.CloudSyncDriftStatus = cmd.Options.CloudSyncDriftStatus
						task.CloudSyncPreview = cmd.Options.CloudSyncPreview
						task.CloudSyncLayer = cmd.Options.CloudSyncLayer
						task.CloudPlanFile = planFile
						task.CloudPlanProvisioner = planProvisioner
						task.CloudPlanRenderTimeout = cmd.Options.CloudPlanRenderTimeout
						task.UseTerragrunt = cmd.Options.UseTerragrunt
						task.EnableSharing = cmd.Options.EnableSharing
						task.MockOnFail = cmd.Options.MockOnFail

						tel.DefaultRecord.Set(
							tel.BoolFlag("sync-deployment", cmd.Options.CloudSyncDeployment),
							tel.BoolFlag("sync-drift", cmd.Options.CloudSyncDriftStatus),
							tel.BoolFlag("sync-preview", cmd.Options.CloudSyncPreview),
							tel.StringFlag("terraform-planfile", cmd.Options.CloudTerraformPlanFile),
							tel.StringFlag("tofu-planfile", cmd.Options.CloudTofuPlanFile),
							tel.StringFlag("layer", string(cmd.Options.CloudSyncLayer)),
							tel.BoolFlag("terragrunt", cmd.Options.UseTerragrunt),
							tel.BoolFlag("output-sharing", cmd.Options.EnableSharing),
							tel.BoolFlag("output-mocks", cmd.Options.MockOnFail),
						)
					}
					run.Tasks = append(run.Tasks, task)
					if task.CloudSyncDeployment || task.CloudSyncDriftStatus || task.CloudSyncPreview {
						run.SyncTaskIndex = len(run.Tasks) - 1
					}
				}
			}

			runs = append(runs, run)
		}
	}

	err = s.prepareScriptForCloudSync(runs)
	if err != nil {
		return err
	}

	err = s.Engine.RunAll(runs, engine.RunAllOptions{
		ScriptRun:       true,
		Quiet:           s.Quiet,
		DryRun:          s.DryRun,
		Reverse:         s.Reverse,
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

func (s *Spec) prepareScriptForCloudSync(runs []engine.StackRun) error {
	if s.DryRun {
		return nil
	}

	deployRuns := engine.SelectCloudStackTasks(runs, engine.IsDeploymentTask)
	driftRuns := engine.SelectCloudStackTasks(runs, engine.IsDriftTask)
	previewRuns := engine.SelectCloudStackTasks(runs, engine.IsPreviewTask)
	if len(deployRuns) == 0 && len(driftRuns) == 0 && len(previewRuns) == 0 {
		return nil
	}

	var feats []string
	if len(deployRuns) > 0 {
		feats = append(feats, cloudFeatScriptSyncDeployment)
	}
	if len(driftRuns) > 0 {
		feats = append(feats, cloudFeatScriptSyncDriftStatus)
	}
	if len(previewRuns) > 0 {
		feats = append(feats, cloudFeatScriptSyncPreview)
	}

	isCI := os.Getenv("GITHUB_ACTIONS") != "" || os.Getenv("GITLAB_CI") != "" || os.Getenv("BITBUCKET_BUILD_NUMBER") != ""
	if len(previewRuns) > 0 && !isCI {
		s.Engine.DisableCloudFeatures(errors.E(cloudSyncPreviewCICDWarning))
		return nil
	}

	if !s.Engine.Project().IsRepo() {
		s.Engine.DisableCloudFeatures(errors.E("cloud features require a git repository"))
		return nil
	}

	err := s.Engine.SetupCloudConfig(feats)
	err = s.Engine.HandleCloudCriticalError(err)
	if err != nil {
		return err
	}

	if s.Engine.IsCloudDisabled() {
		return nil
	}

	if len(deployRuns) > 0 {
		uuid, err := uuid.GenerateUUID()
		err = s.Engine.HandleCloudCriticalError(err)
		if err != nil {
			return err
		}
		s.state.RunUUID = resources.UUID(uuid)
	}

	if s.Engine.IsCloudDisabled() {
		return nil
	}

	cloudsync.DetectCloudMetadata(s.Engine, &s.state)

	if s.Engine.IsCloudDisabled() {
		return nil
	}

	if len(deployRuns) > 0 {
		uuid, err := uuid.GenerateUUID()
		err = s.Engine.HandleCloudCriticalError(err)
		if err != nil {
			return err
		}

		s.state.RunUUID = resources.UUID(uuid)

		sortableDeployStacks := make([]*config.SortableStack, len(deployRuns))
		for i, e := range deployRuns {
			sortableDeployStacks[i] = &config.SortableStack{Stack: e.Stack}
		}
		err = s.Engine.EnsureAllStackHaveIDs(sortableDeployStacks)
		if err != nil {
			return err
		}
		err = cloudsync.CreateCloudDeployment(s.Engine, s.WorkingDir, deployRuns, &s.state)
		if err != nil {
			return err
		}
	}

	if len(driftRuns) > 0 {
		sortableDriftStacks := make([]*config.SortableStack, len(driftRuns))
		for i, e := range driftRuns {
			sortableDriftStacks[i] = &config.SortableStack{Stack: e.Stack}
		}
		err = s.Engine.EnsureAllStackHaveIDs(sortableDriftStacks)
		if err != nil {
			return err
		}
	}

	if len(previewRuns) > 0 {
		for metaID, previewID := range cloudsync.CreateCloudPreview(s.Engine, s.GitFilter, previewRuns, s.Target, s.FromTarget, &s.state) {
			s.state.SetMeta2PreviewID(metaID, previewID)
		}
	}
	return nil
}

func scriptEvalContext(root *config.Root, st *config.Stack, target string) (*eval.Context, error) {
	globalsReport := globals.ForStack(root, st)
	if err := globalsReport.AsError(); err != nil {
		return nil, err
	}

	evalctx := eval.NewContext(stdlib.Functions(st.HostDir(root), root.Tree().Node.Experiments()))
	runtime := root.Runtime()
	runtime.Merge(st.RuntimeValues(root))

	if target != "" {
		runtime["target"] = cty.StringVal(target)
	}

	evalctx.SetNamespace("terramate", runtime)
	evalctx.SetNamespace("global", globalsReport.Globals.AsValueMap())
	evalctx.SetEnv(os.Environ())

	return evalctx, nil
}
