// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"os"
	"os/exec"
	"os/signal"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	prj "github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/run"
	"github.com/terramate-io/terramate/run/dag"
)

const (
	// ErrRunFailed represents the error when the execution fails, whatever the reason.
	ErrRunFailed errors.Kind = "execution failed"

	// ErrRunCanceled represents the error when the execution was canceled.
	ErrRunCanceled errors.Kind = "execution canceled"

	// ErrRunCommandNotFound represents the error when the command cannot be found
	// in the system.
	ErrRunCommandNotFound errors.Kind = "command not found"
)

// ExecContext declares an stack execution context.
type ExecContext struct {
	Stack *config.Stack
	Cmd   []string
}

func (c *cli) runOnStacks() {
	logger := log.With().
		Str("action", "cli.runOnStacks()").
		Str("workingDir", c.wd()).
		Logger()

	c.gitSafeguardDefaultBranchIsReachable()

	if len(c.parsedArgs.Run.Command) == 0 {
		logger.Fatal().Msgf("run expects a cmd")
	}

	c.checkOutdatedGeneratedCode()
	c.checkCloudSync()

	var stacks config.List[*config.SortableStack]
	if c.parsedArgs.Run.NoRecursive {
		st, found, err := config.TryLoadStack(c.cfg(), prj.PrjAbsPath(c.rootdir(), c.wd()))
		if err != nil {
			fatal(err, "loading stack in current directory")
		}

		if !found {
			logger.Fatal().
				Msg("--no-recursive provided but no stack found in the current directory")
		}

		stacks = append(stacks, st.Sortable())
	} else {
		var err error
		stacks, err = c.computeSelectedStacks(true)
		if err != nil {
			fatal(err, "computing selected stacks")
		}
	}

	logger.Trace().Msg("Get order of stacks to run command on.")

	orderedStacks, reason, err := run.Sort(c.cfg(), stacks)
	if err != nil {
		if errors.IsKind(err, dag.ErrCycleDetected) {
			fatal(err, "cycle detected: %s", reason)
		} else {
			fatal(err, "failed to plan execution")
		}
	}

	if c.parsedArgs.Run.Reverse {
		logger.Trace().Msg("Reversing stacks order.")
		config.ReverseStacks(orderedStacks)
	}

	if c.parsedArgs.Run.DryRun {
		logger.Trace().
			Msg("Do a dry run - get order without actually running command.")
		if len(orderedStacks) > 0 {
			c.output.MsgStdOut("The stacks will be executed using order below:")

			for i, s := range orderedStacks {
				stackdir, _ := c.friendlyFmtDir(s.Dir().String())
				c.output.MsgStdOut("\t%d. %s (%s)", i, s.Name, stackdir)
			}
		} else {
			c.output.MsgStdOut("No stacks will be executed.")
		}

		return
	}

	var runStacks []ExecContext
	for _, st := range orderedStacks {
		run := ExecContext{
			Stack: st.Stack,
			Cmd:   c.parsedArgs.Run.Command,
		}
		if c.parsedArgs.Run.Eval {
			run.Cmd = c.evalRunArgs(run.Stack, run.Cmd)
		}
		runStacks = append(runStacks, run)
	}

	if c.parsedArgs.Run.CloudSyncDeployment && c.parsedArgs.Run.CloudSyncDriftStatus {
		fatal(errors.E("--cloud-sync-deployment conflicts with --cloud-sync-drift-status"))
	}

	if c.parsedArgs.Run.CloudSyncDeployment || c.parsedArgs.Run.CloudSyncDriftStatus {
		ensureAllStackHaveIDs(orderedStacks)
	}

	isSuccessExit := func(exitCode int) bool {
		return exitCode == 0
	}

	if c.parsedArgs.Run.CloudSyncDeployment {
		c.createCloudDeployment(runStacks)
	}

	if c.parsedArgs.Run.CloudSyncDriftStatus {
		isSuccessExit = func(exitCode int) bool {
			return exitCode == 0 || exitCode == 2
		}
	}

	err = c.RunAll(runStacks, isSuccessExit)
	if err != nil {
		fatal(err, "one or more commands failed")
	}
}

func ensureAllStackHaveIDs(stacks config.List[*config.SortableStack]) {
	logger := log.With().
		Str("action", "ensureAllStackHaveIDs").
		Logger()

	logger.Trace().Msg("Checking if selected stacks have id")

	var stacksMissingIDs []string
	for _, st := range stacks {
		if st.ID == "" {
			stacksMissingIDs = append(stacksMissingIDs, st.Dir().String())
		}
	}

	if len(stacksMissingIDs) > 0 {
		for _, stackPath := range stacksMissingIDs {
			logger.Error().Str("stack", stackPath).Msg("stack is missing the ID field")
		}
		logger.Warn().Msg("Stacks are missing IDs. You can use 'terramate create --ensure-stack-ids' to add missing IDs to all stacks.")
		fatal(errors.E("The --cloud-sync-deployment flag requires that selected stacks contain an ID field"))
	}
}

// RunAll will execute the list of RunStack definitions. A RunStack defines the
// stack and its command to be executed. The isSuccessCode is a predicate used
// to decide if the command is considered a successful run or not.
// During the execution of this function the default behavior
// for signal handling will be changed so we can wait for the child
// process to exit before exiting Terramate.
// If a single SIGINT is sent to the Terramate process group then Terramate will
// wait for the process graceful exit and abort the execution of all subsequent
// stacks.
// If SIGINT is sent 3x then Terramate will send a SIGKILL to the currently
// running process and abort the execution of all subsequent stacks.
func (c *cli) RunAll(runStacks []ExecContext, isSuccessCode func(exitCode int) bool) error {
	logger := log.With().
		Str("action", "cli.RunAll()").
		Logger()

	errs := errors.L()

	// we load/check the env of all stacks beforehand then no stack is executed
	// if the environment is not correct for all of them.
	stackEnvs, err := c.loadAllStackEnvs(runStacks)
	if err != nil {
		return err
	}

	logger.Trace().Msg("loaded stacks run environment variables, running commands")

	const signalsBufferSize = 10
	signals := make(chan os.Signal, signalsBufferSize)
	signal.Notify(signals, os.Interrupt)
	defer signal.Reset(os.Interrupt)

	cmds := make(chan *exec.Cmd)
	defer close(cmds)

	continueOnError := c.parsedArgs.Run.ContinueOnError
	results := startCmdConsumer(cmds)
	for i, runContext := range runStacks {
		cmdStr := strings.Join(runContext.Cmd, " ")
		logger := log.With().
			Str("cmd", cmdStr).
			Stringer("stack", runContext.Stack).
			Logger()

		c.cloudSyncBefore(runContext.Stack, cmdStr)

		environ := newEnvironFrom(stackEnvs[runContext.Stack.Dir])
		cmdPath, err := run.LookPath(runContext.Cmd[0], environ)
		if err != nil {
			c.cloudSyncAfter(runContext.Stack, -1, errors.E(ErrRunCommandNotFound, err))
			errs.Append(errors.E(err, "running `%s` in stack %s", cmdStr, runContext.Stack.Dir))
			if continueOnError {
				continue
			}
			c.cloudSyncCancelStacks(runStacks[i+1:])
			return errs.AsError()
		}
		cmd := exec.Command(cmdPath, runContext.Cmd[1:]...)
		cmd.Dir = runContext.Stack.HostDir(c.cfg())
		cmd.Env = environ
		cmd.Stdin = c.stdin
		cmd.Stdout = c.stdout
		cmd.Stderr = c.stderr

		logger.Info().Msg("running")

		if err := cmd.Start(); err != nil {
			c.cloudSyncAfter(runContext.Stack, -1, errors.E(err, ErrRunFailed))
			errs.Append(errors.E(err, "running %s (at stack %s)", cmd, runContext.Stack.Dir))
			logger.Error().Err(err).Msg("failed to execute")
			if continueOnError {
				continue
			}
			c.cloudSyncCancelStacks(runStacks[i+1:])
			return errs.AsError()
		}

		cmds <- cmd
		interruptions := 0
		cmdIsRunning := true

		for cmdIsRunning {
			select {
			case sig := <-signals:
				interruptions++

				logger.Info().
					Str("signal", sig.String()).
					Int("interruptions", interruptions).
					Msg("received interruption signal")

				if interruptions >= 3 {
					logger.Info().Msg("interrupted 3x times or more, killing child process")

					if err := cmd.Process.Kill(); err != nil {
						logger.Debug().Err(err).Msg("unable to send kill signal to child process")
					}

					c.cloudSyncAfter(runContext.Stack, -1, errors.E(ErrRunCanceled))
					c.cloudSyncCancelStacks(runStacks[i+1:])

					return errors.E(ErrRunCanceled, "execution aborted by CTRL-C (3x)")
				}
			case result := <-results:
				logger.Trace().Msg("got command result")
				var err error
				if !isSuccessCode(result.cmd.ProcessState.ExitCode()) {
					err = errors.E(result.err, ErrRunFailed, "running %s (at stack %s)", result.cmd, runContext.Stack.Dir)
					errs.Append(err)
					logger.Error().Err(err).Msg("failed to execute")
				}

				c.cloudSyncAfter(runContext.Stack, result.cmd.ProcessState.ExitCode(), err)
				cmdIsRunning = false
			}
		}

		err = errs.AsError()
		if interruptions > 0 || (err != nil && !continueOnError) {
			logger.Info().Msg("interrupting execution of further stacks")

			c.cloudSyncCancelStacks(runStacks[i+1:])
			return errs.AsError()
		}
	}

	return errs.AsError()
}

type cmdResult struct {
	cmd *exec.Cmd
	err error
}

func startCmdConsumer(cmds <-chan *exec.Cmd) <-chan cmdResult {
	results := make(chan cmdResult)
	go func() {
		for cmd := range cmds {
			results <- cmdResult{
				cmd: cmd,
				err: cmd.Wait(),
			}
		}
		close(results)
	}()
	return results
}

func newEnvironFrom(stackEnviron []string) []string {
	environ := make([]string, len(os.Environ()))
	copy(environ, os.Environ())
	environ = append(environ, stackEnviron...)
	return environ
}

func (c *cli) loadAllStackEnvs(runStacks []ExecContext) (map[prj.Path]run.EnvVars, error) {
	logger := log.With().
		Str("action", "cli.loadAllStackEnvs").
		Logger()

	logger.Trace().Msg("loading stacks run environment variables")

	errs := errors.L()
	stackEnvs := map[prj.Path]run.EnvVars{}
	for _, elem := range runStacks {
		env, err := run.LoadEnv(c.cfg(), elem.Stack)
		errs.Append(err)
		stackEnvs[elem.Stack.Dir] = env
	}

	if errs.AsError() != nil {
		return nil, errs.AsError()
	}
	return stackEnvs, nil
}
