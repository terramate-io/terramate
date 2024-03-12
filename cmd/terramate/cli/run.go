// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/printer"
	prj "github.com/terramate-io/terramate/project"
	runutil "github.com/terramate-io/terramate/run"
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

// runContext declares a stack run context.
type runContext struct {
	Stack *config.Stack
	Cmd   []string

	ScriptIdx    int
	ScriptJobIdx int
	ScriptCmdIdx int

	CloudSyncDeployment        bool
	CloudSyncDriftStatus       bool
	CloudSyncTerraformPlanFile string
}

// runResult contains exit code and duration of a completed run.
type runResult struct {
	ExitCode   int
	StartedAt  *time.Time
	FinishedAt *time.Time
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

	reason, err := runutil.Sort(c.cfg(), stacks,
		func(s *config.SortableStack) *config.Stack { return s.Stack })
	if err != nil {
		if errors.IsKind(err, dag.ErrCycleDetected) {
			fatal(err, "cycle detected: %s", reason)
		} else {
			fatal(err, "failed to plan execution")
		}
	}

	if c.parsedArgs.Run.Reverse {
		config.ReverseStacks(stacks)
	}

	if c.parsedArgs.Run.CloudSyncDeployment && c.parsedArgs.Run.CloudSyncDriftStatus {
		fatal(errors.E("--cloud-sync-deployment conflicts with --cloud-sync-drift-status"))
	}

	cloudSyncEnabled := c.parsedArgs.Run.CloudSyncDeployment || c.parsedArgs.Run.CloudSyncDriftStatus

	if c.parsedArgs.Run.CloudSyncTerraformPlanFile != "" && !cloudSyncEnabled {
		fatal(errors.E("--cloud-sync-terraform-plan-file requires flags --cloud-sync-deployment or --cloud-sync-drift-status"))
	}

	if c.parsedArgs.Run.CloudSyncDeployment || c.parsedArgs.Run.CloudSyncDriftStatus {
		if !c.prj.isRepo {
			fatal(errors.E("cloud features requires a git repository"))
		}
		c.ensureAllStackHaveIDs(stacks)
		c.detectCloudMetadata()
	}

	isSuccessExit := func(exitCode int) bool {
		return exitCode == 0
	}

	var runs []runContext
	for _, st := range stacks {
		run := runContext{
			Stack:                      st.Stack,
			Cmd:                        c.parsedArgs.Run.Command,
			CloudSyncDeployment:        c.parsedArgs.Run.CloudSyncDeployment,
			CloudSyncDriftStatus:       c.parsedArgs.Run.CloudSyncDriftStatus,
			CloudSyncTerraformPlanFile: c.parsedArgs.Run.CloudSyncTerraformPlanFile,
		}
		if c.parsedArgs.Run.Eval {
			run.Cmd, err = c.evalRunArgs(run.Stack, run.Cmd)
			if err != nil {
				c.fatal("unable to evaluate command", err)
			}
		}
		runs = append(runs, run)
	}

	if c.parsedArgs.Run.CloudSyncDeployment {
		c.createCloudDeployment(runs)
	}

	if c.parsedArgs.Run.CloudSyncDriftStatus {
		isSuccessExit = func(exitCode int) bool {
			return exitCode == 0 || exitCode == 2
		}
	}

	err = c.runAll(runs, isSuccessExit, runAllOptions{
		Quiet:           c.parsedArgs.Quiet,
		DryRun:          c.parsedArgs.Run.DryRun,
		ScriptRun:       false,
		ContinueOnError: c.parsedArgs.Run.ContinueOnError,
	})
	if err != nil {
		c.fatal("one or more commands failed", err)
	}
}

// RunAllOptions define named flags for RunAll
type runAllOptions struct {
	Quiet           bool
	DryRun          bool
	ScriptRun       bool
	ContinueOnError bool
}

// runAll will execute the list of RunStack definitions. A RunStack defines the
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
func (c *cli) runAll(
	runs []runContext,
	isSuccessCode func(exitCode int) bool,
	opts runAllOptions,
) error {
	errs := errors.L()

	// we load/check the env of all stacks beforehand then no stack is executed
	// if the environment is not correct for all of them.
	stackEnvs, err := c.loadAllStackEnvs(runs)
	if err != nil {
		return err
	}

	const signalsBufferSize = 10
	signals := make(chan os.Signal, signalsBufferSize)
	signal.Notify(signals, os.Interrupt)
	defer signal.Reset(os.Interrupt)

	cmds := make(chan *exec.Cmd)
	defer close(cmds)

	continueOnError := opts.ContinueOnError
	results := startCmdConsumer(cmds)
	printPrefix := "terramate:"
	if !opts.ScriptRun && opts.DryRun {
		printPrefix = fmt.Sprintf("%s (dry-run)", printPrefix)
	}

	for i, run := range runs {
		cmdStr := strings.Join(run.Cmd, " ")
		logger := log.With().
			Str("cmd", cmdStr).
			Stringer("stack", run.Stack).
			Logger()

		if opts.ScriptRun {
			printScriptCommand(c.stderr, run)
		}

		c.cloudSyncBefore(run)

		environ := newEnvironFrom(stackEnvs[run.Stack.Dir])
		cmdPath, err := runutil.LookPath(run.Cmd[0], environ)
		if err != nil {
			c.cloudSyncAfter(run, runResult{ExitCode: -1}, errors.E(ErrRunCommandNotFound, err))
			errs.Append(errors.E(err, "running `%s` in stack %s", cmdStr, run.Stack.Dir))
			if continueOnError {
				continue
			}
			c.cloudSyncCancelStacks(runs[i+1:])
			return errs.AsError()
		}

		if !opts.Quiet && !opts.ScriptRun {
			printer.Stderr.Println(printPrefix + " Entering stack in " + run.Stack.String())
			printer.Stderr.Println(printPrefix + " Executing command " + strconv.Quote(cmdStr))
		}

		if opts.DryRun {
			continue
		}

		cmd := exec.Command(cmdPath, run.Cmd[1:]...)
		cmd.Dir = run.Stack.HostDir(c.cfg())
		cmd.Env = environ

		stdout := c.stdout
		stderr := c.stderr

		logSyncWait := func() {}
		if c.cloudEnabled() && run.CloudSyncDeployment {
			logSyncer := cloud.NewLogSyncer(func(logs cloud.DeploymentLogs) {
				c.syncLogs(&logger, run, logs)
			})
			stdout = logSyncer.NewBuffer(cloud.StdoutLogChannel, c.stdout)
			stderr = logSyncer.NewBuffer(cloud.StderrLogChannel, c.stderr)

			logSyncWait = logSyncer.Wait
		}

		cmd.Stdin = c.stdin
		cmd.Stdout = stdout
		cmd.Stderr = stderr

		startTime := time.Now().UTC()

		if err := cmd.Start(); err != nil {
			endTime := time.Now().UTC()

			logSyncWait()

			res := runResult{
				ExitCode:   -1,
				StartedAt:  &startTime,
				FinishedAt: &endTime,
			}
			c.cloudSyncAfter(run, res, errors.E(err, ErrRunFailed))
			errs.Append(errors.E(err, "running %s (at stack %s)", cmd, run.Stack.Dir))
			if continueOnError {
				continue
			}
			c.cloudSyncCancelStacks(runs[i+1:])
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

					endTime := time.Now().UTC()

					logSyncWait()

					res := runResult{
						ExitCode:   -1,
						StartedAt:  &startTime,
						FinishedAt: &endTime,
					}
					c.cloudSyncAfter(run, res, errors.E(ErrRunCanceled))
					c.cloudSyncCancelStacks(runs[i+1:])
					return errors.E(ErrRunCanceled, "execution aborted by CTRL-C (3x)")
				}
			case result := <-results:
				logSyncWait()
				var err error
				if !isSuccessCode(result.cmd.ProcessState.ExitCode()) {
					err = errors.E(result.err, ErrRunFailed, "running %s (in %s)", result.cmd, run.Stack.Dir)
					errs.Append(err)
				}

				res := runResult{
					ExitCode:   result.cmd.ProcessState.ExitCode(),
					StartedAt:  &startTime,
					FinishedAt: result.finishedAt,
				}

				logMsg := logger.Debug().Int("exit_code", res.ExitCode)
				if res.StartedAt != nil && res.FinishedAt != nil {
					logMsg = logMsg.
						Time("started_at", *res.StartedAt).
						Time("finished_at", *res.FinishedAt).
						TimeDiff("duration", *res.FinishedAt, *res.StartedAt)
				}
				logMsg.Msg("command execution finished")

				c.cloudSyncAfter(run, res, err)
				cmdIsRunning = false
			}
		}

		err = errs.AsError()
		if interruptions > 0 || (err != nil && !continueOnError) {
			logger.Info().Msg("interrupting execution of further stacks")

			c.cloudSyncCancelStacks(runs[i+1:])
			return errs.AsError()
		}
	}

	return errs.AsError()
}

func (c *cli) syncLogs(logger *zerolog.Logger, run runContext, logs cloud.DeploymentLogs) {
	data, _ := json.Marshal(logs)
	logger.Debug().RawJSON("logs", data).Msg("synchronizing logs")
	ctx, cancel := context.WithTimeout(context.Background(), defaultCloudTimeout)
	defer cancel()
	stackID := c.cloud.run.meta2id[strings.ToLower(run.Stack.ID)]
	err := c.cloud.client.SyncDeploymentLogs(
		ctx, c.cloud.run.orgUUID, stackID, c.cloud.run.runUUID, logs,
	)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to sync logs")
	}
}

type cmdResult struct {
	cmd        *exec.Cmd
	err        error
	finishedAt *time.Time
}

func startCmdConsumer(cmds <-chan *exec.Cmd) <-chan cmdResult {
	results := make(chan cmdResult)
	go func() {
		for cmd := range cmds {
			err := cmd.Wait()
			endTime := time.Now().UTC()

			results <- cmdResult{
				cmd:        cmd,
				err:        err,
				finishedAt: &endTime,
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

func (c *cli) loadAllStackEnvs(runs []runContext) (map[prj.Path]runutil.EnvVars, error) {
	errs := errors.L()
	stackEnvs := map[prj.Path]runutil.EnvVars{}
	for _, run := range runs {
		env, err := runutil.LoadEnv(c.cfg(), run.Stack)
		errs.Append(err)
		stackEnvs[run.Stack.Dir] = env
	}

	if errs.AsError() != nil {
		return nil, errs.AsError()
	}
	return stackEnvs, nil
}
