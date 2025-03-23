// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/hashicorp/go-uuid"
	"github.com/rs/zerolog"
	"github.com/terramate-io/terramate/cloud/api/resources"
	"github.com/terramate-io/terramate/cloud/api/status"
	"github.com/terramate-io/terramate/cloudsync"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/config/filter"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/globals"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/printer"
	prj "github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/stdlib"
	tel "github.com/terramate-io/terramate/ui/tui/telemetry"
	"github.com/zclconf/go-cty/cty"
)

const (
	cloudFeatScriptSyncDeployment  = "Script option 'sync_deployment' is a Terramate Cloud feature to synchronize deployment details to Terramate Cloud."
	cloudFeatScriptSyncDriftStatus = "Script option 'sync_drift_status' is a Terramate Cloud feature to synchronize drift and health check results to Terramate Cloud."
	cloudFeatScriptSyncPreview     = "Script option 'sync_preview' is a Terramate Cloud feature to synchronize deployment previews to Terramate Cloud."
)

func (c *cli) runScript() {
	c.checkOutdatedGeneratedCode()

	err := c.engine.CheckTargetsConfiguration(c.parsedArgs.Script.Run.Target, c.parsedArgs.Script.Run.FromTarget, func(isTargetSet bool) error {
		if !isTargetSet {
			// We don't check here if any script has any sync command options enabled.
			// We assume yes and so --target must be set.
			return errors.E("--target is required when terramate.config.cloud.targets.enabled is true")
		}
		return nil
	})
	if err := c.engine.HandleCloudCriticalError(err); err != nil {
		fatal(err)
	}

	var stacks config.List[*config.SortableStack]
	if c.parsedArgs.Script.Run.NoRecursive {
		st, found, err := config.TryLoadStack(c.cfg(), prj.PrjAbsPath(c.rootdir(), c.wd()))
		if err != nil {
			fatalWithDetailf(err, "failed to load stack in current directory")
		}

		if !found {
			fatal("--no-recursive provided but no stack found in the current directory")
		}

		stacks = append(stacks, st.Sortable())
	} else {
		var err error
		stackFilters, err := status.ParseFilters(c.parsedArgs.Script.Run.Status, c.parsedArgs.Script.Run.DeploymentStatus, c.parsedArgs.Script.Run.DriftStatus)
		if err != nil {
			fatalWithDetailf(err, "failed to parse stack filters")
		}

		gitfilter, err := engine.NewGitFilter(c.parsedArgs.Changed, c.parsedArgs.GitChangeBase, c.parsedArgs.Script.Run.EnableChangeDetection, c.parsedArgs.Script.Run.DisableChangeDetection)
		if err != nil {
			fatalWithDetailf(err, "failed to create git filter")
		}

		tagsFilter, err := filter.ParseTags(c.parsedArgs.Tags, c.parsedArgs.NoTags)
		if err != nil {
			fatalWithDetailf(err, "failed to parse tags")
		}

		stacks, err = c.engine.ComputeSelectedStacks(
			gitfilter,
			tagsFilter,
			engine.OutputsSharingOptions{
				IncludeOutputDependencies: c.parsedArgs.Script.Run.IncludeOutputDependencies,
				OnlyOutputDependencies:    c.parsedArgs.Script.Run.OnlyOutputDependencies,
			},
			c.parsedArgs.Script.Run.Target,
			stackFilters,
		)
		if err != nil {
			fatalWithDetailf(err, "failed to compute selected stacks")
		}

		if !c.parsedArgs.Script.Run.DryRun {
			err := gitFileSafeguards(c.engine, true, c.safeguards)
			if err != nil {
				fatal(err)
			}
		}
	}

	c.gitSafeguardDefaultBranchIsReachable()

	// search for the script and prepare a list of script/stack entries
	m := newScriptsMatcher(c.parsedArgs.Script.Run.Cmds)
	m.Search(c.cfg(), stacks)

	if len(m.Results) == 0 {
		c.output.MsgStdErr(color.RedString("script not found: ") +
			strings.Join(c.parsedArgs.Script.Run.Cmds, " "))
		os.Exit(1)
	}

	if c.parsedArgs.Script.Run.DryRun {
		c.output.MsgStdErr("This is a dry run, commands will not be executed.")
	}

	var runs []engine.StackRun

	for scriptIdx, result := range m.Results {
		if len(result.Stacks) == 0 {
			continue
		}

		if !c.parsedArgs.Quiet {
			c.output.MsgStdErr("Script %s at %s having %s job(s)",
				color.GreenString(fmt.Sprintf("%d", scriptIdx)),
				color.BlueString(result.ScriptCfg.Range.String()),
				color.BlueString(fmt.Sprintf("%d", len(result.ScriptCfg.Jobs))),
			)
		}

		for _, st := range result.Stacks {
			run := engine.StackRun{Stack: st.Stack}

			ectx, err := scriptEvalContext(c.cfg(), st.Stack, c.parsedArgs.Script.Run.Target)
			if err != nil {
				fatalWithDetailf(err, "failed to get context")
			}

			evalScript, err := config.EvalScript(ectx, *result.ScriptCfg)
			if err != nil {
				fatalWithDetailf(err, "failed to eval script")
			}

			for jobIdx, job := range evalScript.Jobs {
				for cmdIdx, cmd := range job.Commands() {
					task := engine.StackRunTask{
						Cmd:             cmd.Args,
						CloudTarget:     c.parsedArgs.Script.Run.Target,
						CloudFromTarget: c.parsedArgs.Script.Run.FromTarget,
						ScriptIdx:       scriptIdx,
						ScriptJobIdx:    jobIdx,
						ScriptCmdIdx:    cmdIdx,
					}

					if cmd.Options != nil {
						planFile, planProvisioner := selectPlanFile(
							cmd.Options.CloudTerraformPlanFile,
							cmd.Options.CloudTofuPlanFile)

						task.CloudSyncDeployment = cmd.Options.CloudSyncDeployment
						task.CloudSyncDriftStatus = cmd.Options.CloudSyncDriftStatus
						task.CloudSyncPreview = cmd.Options.CloudSyncPreview
						task.CloudSyncLayer = cmd.Options.CloudSyncLayer
						task.CloudPlanFile = planFile
						task.CloudPlanProvisioner = planProvisioner
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

	var cloudState cloudsync.CloudRunState
	c.prepareScriptForCloudSync(runs, &cloudState)

	err = c.engine.RunAll(runs, engine.RunAllOptions{
		Quiet:           c.parsedArgs.Quiet,
		DryRun:          c.parsedArgs.Script.Run.DryRun,
		Reverse:         c.parsedArgs.Script.Run.Reverse,
		ScriptRun:       true,
		ContinueOnError: c.parsedArgs.Script.Run.ContinueOnError,
		Parallel:        c.parsedArgs.Script.Run.Parallel,
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

func (c *cli) prepareScriptForCloudSync(runs []engine.StackRun, state *cloudsync.CloudRunState) {
	if c.parsedArgs.Script.Run.DryRun {
		return
	}

	deployRuns := engine.SelectCloudStackTasks(runs, engine.IsDeploymentTask)
	driftRuns := engine.SelectCloudStackTasks(runs, engine.IsDriftTask)
	previewRuns := engine.SelectCloudStackTasks(runs, engine.IsPreviewTask)
	if len(deployRuns) == 0 && len(driftRuns) == 0 && len(previewRuns) == 0 {
		return
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
		printer.Stderr.Warn(cloudSyncPreviewCICDWarning)
		c.engine.DisableCloudFeatures(errors.E(cloudSyncPreviewCICDWarning))
		return
	}

	prj := c.project()
	if !prj.IsRepo() {
		c.engine.DisableCloudFeatures(errors.E("cloud features require a git repository"))
		return
	}

	err := c.engine.SetupCloudConfig(feats)
	if err := c.engine.HandleCloudCriticalError(err); err != nil {
		fatal(err)
	}

	if c.engine.IsCloudDisabled() {
		return
	}

	cloudsync.DetectCloudMetadata(c.engine, state)

	if len(deployRuns) > 0 {
		uuid, err := uuid.GenerateUUID()
		if err := c.engine.HandleCloudCriticalError(err); err != nil {
			fatal(err)
		} else {
			state.RunUUID = resources.UUID(uuid)
		}

		sortableDeployStacks := make([]*config.SortableStack, len(deployRuns))
		for i, e := range deployRuns {
			sortableDeployStacks[i] = &config.SortableStack{Stack: e.Stack}
		}
		c.ensureAllStackHaveIDs(sortableDeployStacks)
		err = cloudsync.CreateCloudDeployment(c.engine, c.wd(), deployRuns, state)
		if err := c.engine.HandleCloudCriticalError(err); err != nil {
			fatal(err)
		}
		return
	}

	if len(driftRuns) > 0 {
		sortableDriftStacks := make([]*config.SortableStack, len(driftRuns))
		for i, e := range driftRuns {
			sortableDriftStacks[i] = &config.SortableStack{Stack: e.Stack}
		}
		c.ensureAllStackHaveIDs(sortableDriftStacks)
	}

	if len(previewRuns) > 0 {
		gitfilter, err := engine.NewGitFilter(c.parsedArgs.Changed, c.parsedArgs.GitChangeBase, c.parsedArgs.Script.Run.EnableChangeDetection, c.parsedArgs.Script.Run.DisableChangeDetection)
		if err != nil {
			fatal(err)
		}
		// HACK: Target and FromTarget are passed through opts for preview and not used from the runs.
		for metaID, previewID := range cloudsync.CreateCloudPreview(c.engine, gitfilter, previewRuns, c.parsedArgs.Script.Run.Target, c.parsedArgs.Script.Run.FromTarget, state) {
			state.SetMeta2PreviewID(metaID, previewID)
		}
	}
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

func (c *cli) checkScriptEnabled() {
	if !c.cfg().HasExperiment("scripts") {
		printer.Stderr.Error(`The "scripts" feature is not enabled`)
		printer.Stderr.Println(`In order to enable it you must set the terramate.config.experiments attribute.`)
		printer.Stderr.Println(`Example:

terramate {
  config {
    experiments = ["scripts"]
  }
}`)
		os.Exit(1)
	}
}
