// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"os/exec"

	"github.com/fatih/color"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/globals"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/printer"
	prj "github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/run"
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

	for _, result := range m.Results {
		if len(result.Stacks) == 0 {
			continue
		}

		c.output.MsgStdErr("Found %s defined at %s having %s job(s)",
			color.GreenString(result.ScriptCfg.Name()),
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

			for jobNum, j := range evalScript.Jobs {
				c.output.MsgStdOut("")
				for cmdNum, cmd := range j.Commands() {
					printCommand(c.stderr, cmd, st.Dir().String(), jobNum, cmdNum)

					env, err := run.LoadEnv(c.cfg(), st.Stack)
					if err != nil {
						logger.Fatal().Err(err).Msg("failed to load env")
					}

					if err := c.executeCommand(cmd, st.Dir().HostPath(c.rootdir()), newEnvironFrom(env)); err != nil {
						logger.Fatal().Err(err).Msg("unable to execute command")
					}
				}
			}
		}
	}
}

func (c *cli) executeCommand(cmd []string, wd string, env []string) error {
	if c.parsedArgs.Script.Run.DryRun {
		return nil
	}

	newCmd, err := makeCommand(cmd, wd, env, c.stdout, c.stderr)
	if err != nil {
		return errors.E(err, "failed to prepare command")
	}

	if err := newCmd.Run(); err != nil {
		return errors.E(err, "failed to execute command")
	}

	return nil
}

func makeCommand(command []string, dir string, env []string, stdout, stderr io.Writer) (*exec.Cmd, error) {
	cmdPath, err := run.LookPath(command[0], env)
	if err != nil {
		return nil, errors.E(err, "%s: command not found", command[0])
	}

	runCmd := exec.Command(cmdPath, command[1:]...)
	runCmd.Dir = dir
	runCmd.Env = env
	runCmd.Stdout = stdout
	runCmd.Stderr = stderr

	return runCmd, nil
}

// printCommand pretty prints the cmd and attaches a "prompt" style prefix to it
// for example:
// /somestack (job:0.0)> echo hello
func printCommand(w io.Writer, cmd []string, wd string, jobNum, cmdNum int) {
	prompt := color.GreenString(fmt.Sprintf("%s (job:%d.%d)>", wd, jobNum, cmdNum))
	fmt.Fprintln(w, prompt, color.YellowString(strings.Join(cmd, " ")))
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
		printer.Stderr.Errorln(`The "scripts" feature is not enabled`)
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
