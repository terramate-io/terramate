// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/globals"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/printer"
	prj "github.com/terramate-io/terramate/project"
	runutil "github.com/terramate-io/terramate/run"
	"github.com/terramate-io/terramate/run/dag"
	"github.com/terramate-io/terramate/stdlib"
)

func (c *cli) runScript() {
	logger := log.With().
		Str("action", "cli.runScript()").
		Str("workingDir", c.wd()).
		Logger()

	c.gitSafeguardDefaultBranchIsReachable()
	c.checkOutdatedGeneratedCode()

	var stacks config.List[*config.SortableStack]
	if c.parsedArgs.Script.Run.NoRecursive {
		st, found, err := config.TryLoadStack(c.cfg(), prj.PrjAbsPath(c.rootdir(), c.wd()))
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to load stack in current directory")
		}

		if !found {
			logger.Fatal().Msg("--no-recursive provided but no stack found in the current directory")
		}

		stacks = append(stacks, st.Sortable())
	} else {
		var err error
		stacks, err = c.computeSelectedStacks(true)
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to compute selected stacks")
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

	var runs []runContext

	for scriptIdx, result := range m.Results {
		if len(result.Stacks) == 0 {
			continue
		}

		c.output.MsgStdErr("Script %s at %s having %s job(s)",
			color.GreenString(fmt.Sprintf("%d", scriptIdx)),
			color.BlueString(result.ScriptCfg.Range.String()),
			color.BlueString(fmt.Sprintf("%d", len(result.ScriptCfg.Jobs))),
		)

		for _, st := range result.Stacks {
			ectx, err := scriptEvalContext(c.cfg(), st.Stack)
			if err != nil {
				logger.Fatal().Err(err).Msg("failed to get context")
			}

			evalScript, err := config.EvalScript(ectx, *result.ScriptCfg)
			if err != nil {
				logger.Fatal().Err(err).Msg("failed to eval script")
			}

			for jobIdx, job := range evalScript.Jobs {
				for cmdIdx, cmd := range job.Commands() {
					exc := runContext{
						Stack:        st.Stack,
						Cmd:          cmd.Args,
						ScriptIdx:    scriptIdx,
						ScriptJobIdx: jobIdx,
						ScriptCmdIdx: cmdIdx,
					}

					if cmd.Options != nil {
						exc.CloudSyncDeployment = cmd.Options.CloudSyncDeployment
						exc.CloudSyncTerraformPlanFile = cmd.Options.CloudSyncTerraformPlan
					}

					runs = append(runs, exc)
				}
			}
		}
	}

	orderedStacks, reason, err := runutil.Sort(c.cfg(), stacks)
	if err != nil {
		if errors.IsKind(err, dag.ErrCycleDetected) {
			fatal(err, "cycle detected: %s", reason)
		} else {
			fatal(err, "failed to plan execution")
		}
	}

	var orderedRuns []runContext
	for _, st := range orderedStacks {
		for _, r := range runs {
			if r.Stack.Dir.String() == st.Dir().String() {
				orderedRuns = append(orderedRuns, r)
			}
		}
	}

	c.prepareScriptCloudDeploymentSync(orderedRuns)

	isSuccessExit := func(exitCode int) bool {
		return exitCode == 0
	}

	err = c.runAll(orderedRuns, isSuccessExit, runAllOptions{
		Quiet:           c.parsedArgs.Quiet,
		DryRun:          c.parsedArgs.Script.Run.DryRun,
		ScriptRun:       true,
		ContinueOnError: false,
	})
	if err != nil {
		c.fatal("one or more commands failed", err)
	}
}

func (c *cli) prepareScriptCloudDeploymentSync(runStacks []runContext) {
	if c.parsedArgs.Script.Run.DryRun {
		return
	}

	var deployRuns []runContext
	for _, exc := range runStacks {
		if exc.CloudSyncDeployment {
			deployRuns = append(deployRuns, exc)
		}
	}

	if len(deployRuns) == 0 {
		return
	}

	if !c.prj.isRepo {
		fatal(errors.E("cloud features require a git repository"))
	}

	err := c.setupCloudConfig()
	c.handleCriticalError(err)

	if c.cloud.disabled {
		return
	}

	c.cloud.run.meta2id = make(map[string]int64)
	uuid, err := generateRunID()
	c.handleCriticalError(err)
	c.cloud.run.runUUID = cloud.UUID(uuid)

	c.detectCloudMetadata()

	sortableDeployStacks := make([]*config.SortableStack, len(deployRuns))
	for i, e := range deployRuns {
		sortableDeployStacks[i] = &config.SortableStack{Stack: e.Stack}
	}
	c.ensureAllStackHaveIDs(sortableDeployStacks)

	c.createCloudDeployment(deployRuns)
}

// printScriptCommand pretty prints the cmd and attaches a "prompt" style prefix to it
// for example:
// /somestack (script:0 job:0.0)> echo hello
func printScriptCommand(w io.Writer, run runContext) {
	prompt := color.GreenString(fmt.Sprintf("%s (script:%d job:%d.%d)>",
		run.Stack.Dir.String(),
		run.ScriptIdx, run.ScriptJobIdx, run.ScriptCmdIdx))
	fmt.Fprintln(w, prompt, color.YellowString(strings.Join(run.Cmd, " ")))
}

func scriptEvalContext(root *config.Root, st *config.Stack) (*eval.Context, error) {
	globalsReport := globals.ForStack(root, st)
	if err := globalsReport.AsError(); err != nil {
		return nil, err
	}

	evalctx := eval.NewContext(stdlib.Functions(st.HostDir(root)))
	runtime := root.Runtime()
	runtime.Merge(st.RuntimeValues(root))
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
