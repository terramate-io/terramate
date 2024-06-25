// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/hashicorp/go-uuid"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/globals"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/printer"
	prj "github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/stdlib"
	"github.com/zclconf/go-cty/cty"
)

const (
	cloudFeatScriptSyncDeployment  = "Script option 'sync_deployment' is a Terramate Cloud feature to synchronize deployment details to Terramate Cloud."
	cloudFeatScriptSyncDriftStatus = "Script option 'sync_drift_status' is a Terramate Cloud feature to synchronize drift and health check results to Terramate Cloud."
	cloudFeatScriptSyncPreview     = "Script option 'sync_preview' is a Terramate Cloud feature to synchronize deployment previews to Terramate Cloud."
)

func (c *cli) runScript() {
	c.gitSafeguardDefaultBranchIsReachable()
	c.checkOutdatedGeneratedCode()

	c.checkTargetsConfiguration(c.parsedArgs.Script.Run.Target, c.parsedArgs.Script.Run.FromTarget, func(isTargetSet bool) {
		if !isTargetSet {
			// We don't check here if any script has any sync command options enabled.
			// We assume yes and so --target must be set.
			fatal("--target is required when terramate.config.cloud.targets.enabled is true")
		}
	})

	var stacks config.List[*config.SortableStack]
	if c.parsedArgs.Script.Run.NoRecursive {
		st, found, err := config.TryLoadStack(c.cfg(), prj.PrjAbsPath(c.rootdir(), c.wd()))
		if err != nil {
			fatalWithDetails(err, "failed to load stack in current directory")
		}

		if !found {
			fatal("--no-recursive provided but no stack found in the current directory")
		}

		stacks = append(stacks, st.Sortable())
	} else {
		var err error
		statusFilter := parseStatusFilter(c.parsedArgs.Script.Run.Status)
		deploymentFilter := parseDeploymentStatusFilter(c.parsedArgs.Script.Run.DeploymentStatus)
		driftFilter := parseDriftStatusFilter(c.parsedArgs.Script.Run.DriftStatus)
		stacks, err = c.computeSelectedStacks(true, c.parsedArgs.Script.Run.Target, cloud.StatusFilters{
			StackStatus:      statusFilter,
			DeploymentStatus: deploymentFilter,
			DriftStatus:      driftFilter,
		})
		if err != nil {
			fatalWithDetails(err, "failed to compute selected stacks")
		}
	}

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

	var runs []stackRun

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
			run := stackRun{Stack: st.Stack}

			ectx, err := scriptEvalContext(c.cfg(), st.Stack, c.parsedArgs.Script.Run.Target)
			if err != nil {
				fatalWithDetails(err, "failed to get context")
			}

			evalScript, err := config.EvalScript(ectx, *result.ScriptCfg)
			if err != nil {
				fatalWithDetails(err, "failed to eval script")
			}

			for jobIdx, job := range evalScript.Jobs {
				for cmdIdx, cmd := range job.Commands() {
					task := stackRunTask{
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
					}
					run.Tasks = append(run.Tasks, task)
				}
			}

			runs = append(runs, run)
		}
	}

	c.prepareScriptForCloudSync(runs)

	err := c.runAll(runs, runAllOptions{
		Quiet:           c.parsedArgs.Quiet,
		DryRun:          c.parsedArgs.Script.Run.DryRun,
		Reverse:         c.parsedArgs.Script.Run.Reverse,
		ScriptRun:       true,
		ContinueOnError: c.parsedArgs.Script.Run.ContinueOnError,
		Parallel:        c.parsedArgs.Script.Run.Parallel,
	})
	if err != nil {
		fatalWithDetails(err, "one or more commands failed")
	}
}

func (c *cli) prepareScriptForCloudSync(runs []stackRun) {
	if c.parsedArgs.Script.Run.DryRun {
		return
	}

	deployRuns := selectCloudStackTasks(runs, isDeploymentTask)
	driftRuns := selectCloudStackTasks(runs, isDriftTask)
	previewRuns := selectCloudStackTasks(runs, isPreviewTask)
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

	if len(previewRuns) > 0 && os.Getenv("GITHUB_ACTIONS") == "" {
		printer.Stderr.Warn(cloudSyncPreviewGHAWarning)
		c.disableCloudFeatures(errors.E(cloudSyncPreviewGHAWarning))
	}

	if !c.prj.isRepo {
		c.handleCriticalError(errors.E("cloud features require a git repository"))
		return
	}

	err := c.setupCloudConfig(feats)
	c.handleCriticalError(err)

	if c.cloud.disabled {
		return
	}

	c.detectCloudMetadata()

	if len(deployRuns) > 0 {
		uuid, err := uuid.GenerateUUID()
		c.handleCriticalError(err)
		c.cloud.run.runUUID = cloud.UUID(uuid)

		sortableDeployStacks := make([]*config.SortableStack, len(deployRuns))
		for i, e := range deployRuns {
			sortableDeployStacks[i] = &config.SortableStack{Stack: e.Stack}
		}
		c.ensureAllStackHaveIDs(sortableDeployStacks)
		c.createCloudDeployment(deployRuns)
	}

	if len(driftRuns) > 0 {
		sortableDriftStacks := make([]*config.SortableStack, len(driftRuns))
		for i, e := range driftRuns {
			sortableDriftStacks[i] = &config.SortableStack{Stack: e.Stack}
		}
		c.ensureAllStackHaveIDs(sortableDriftStacks)
	}

	if len(previewRuns) > 0 {
		// HACK: Target and FromTarget are passed through opts for preview and not used from the runs.
		for metaID, previewID := range c.createCloudPreview(previewRuns, c.parsedArgs.Script.Run.Target, c.parsedArgs.Script.Run.FromTarget) {
			c.cloud.run.setMeta2PreviewID(metaID, previewID)
		}
	}
}

// printScriptCommand pretty prints the cmd and attaches a "prompt" style prefix to it
// for example:
// /somestack (script:0 job:0.0)> echo hello
func printScriptCommand(w io.Writer, stack *config.Stack, run stackRunTask) {
	prompt := color.GreenString(fmt.Sprintf("%s (script:%d job:%d.%d)>",
		stack.Dir.String(),
		run.ScriptIdx, run.ScriptJobIdx, run.ScriptCmdIdx))
	fmt.Fprintln(w, prompt, color.YellowString(strings.Join(run.Cmd, " ")))
}

func scriptEvalContext(root *config.Root, st *config.Stack, target string) (*eval.Context, error) {
	globalsReport := globals.ForStack(root, st)
	if err := globalsReport.AsError(); err != nil {
		return nil, err
	}

	evalctx := eval.NewContext(stdlib.Functions(st.HostDir(root)))
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
