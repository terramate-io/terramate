// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package engine

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"time"

	stdjson "encoding/json"

	"github.com/fatih/color"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/preview"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/run"
	runutil "github.com/terramate-io/terramate/run"
	"github.com/terramate-io/terramate/run/dag"
	"github.com/terramate-io/terramate/scheduler"
	"github.com/terramate-io/terramate/scheduler/resource"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/json"
)

const (
	// ErrRunFailed represents the error when the execution fails, whatever the reason.
	ErrRunFailed errors.Kind = "execution failed"

	// ErrRunCanceled represents the error when the execution was canceled.
	ErrRunCanceled errors.Kind = "execution canceled"

	// ErrRunCommandNotExecuted represents the error when the command was not executed for whatever reason.
	ErrRunCommandNotExecuted errors.Kind = "command not found"
)

// stackRun contains a list of tasks to be run per stack.
type stackRun struct {
	Stack         *config.Stack
	Tasks         []stackRunTask
	SyncTaskIndex int // index of the task with sync options
}

type stackRunTask struct {
	Cmd []string

	ScriptIdx    int
	ScriptJobIdx int
	ScriptCmdIdx int

	CloudTarget     string
	CloudFromTarget string

	CloudSyncDeployment  bool
	CloudSyncDriftStatus bool
	CloudSyncPreview     bool
	CloudSyncLayer       preview.Layer

	CloudPlanFile        string
	CloudPlanProvisioner string

	UseTerragrunt bool
	EnableSharing bool
	MockOnFail    bool
}

type runAllOptions struct {
	Quiet           bool
	DryRun          bool
	Reverse         bool
	ScriptRun       bool
	ContinueOnError bool
	Parallel        int

	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader
}

// stackCloudRun is a stackRun, but with a single task, because the cloud API only supports
// a single command per stack for any operation (deploy, drift, preview).
type stackCloudRun struct {
	Target string
	Stack  *config.Stack
	Task   stackRunTask
	Env    []string
}

// runResult contains exit code and duration of a completed run.
type runResult struct {
	ExitCode   int
	StartedAt  *time.Time
	FinishedAt *time.Time
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
func (e *Engine) RunAll(
	runs []stackRun,
	opts runAllOptions,
) error {
	// Construct a DAG from the list of stackRuns, based on the implicit and
	// explicit dependencies between stacks.
	d, reason, err := runutil.BuildDAGFromStacks(e.Config(), runs,
		func(run stackRun) *config.Stack { return run.Stack })
	if err != nil {
		if errors.IsKind(err, dag.ErrCycleDetected) {
			return errors.E(err, "cycle detected: %s", reason)
		} else {
			return errors.E(err, "failed to plan execution")
		}
	}

	// This context is used to cancel execution mid-progress and skip pending runs.
	// It will not abort any already started runs.
	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// This context is used to kill running processes.
	killCtx, kill := context.WithCancel(context.Background())
	defer kill()

	// Select a scheduling strategy for the DAG nodes.
	var sched scheduler.S[stackRun]
	acquireResource := func() {}
	releaseResource := func() {}

	if opts.Parallel > 1 {
		sched = scheduler.NewParallel(d, opts.Reverse)

		rg := resource.NewBounded(opts.Parallel)
		// Acquire can fail, but not with context.Background().
		acquireResource = func() { _ = rg.Acquire(context.Background()) }
		releaseResource = func() { rg.Release() }
	} else {
		sched = scheduler.NewSequential(d, opts.Reverse)
	}

	// we load/check the env of all stacks beforehand then no stack is executed
	// if the environment is not correct for all of them.
	stackEnvs, err := loadAllStackEnvs(e.Config(), runs)
	if err != nil {
		return err
	}

	const signalsBufferSize = 10
	signals := make(chan os.Signal, signalsBufferSize)
	signal.Notify(signals, os.Interrupt)
	defer signal.Reset(os.Interrupt)

	continueOnError := opts.ContinueOnError

	printPrefix := "terramate:"
	if !opts.ScriptRun && opts.DryRun {
		printPrefix = fmt.Sprintf("%s (dry-run)", printPrefix)
	}

	go func() {
		interruptions := 0

		logger := log.With().Logger()

		for {
			select {
			case <-killCtx.Done():
				return

			case sig := <-signals:
				interruptions++

				logger.Info().
					Str("signal", sig.String()).
					Int("interruptions", interruptions).
					Msg("received interruption signal")

				logger.Info().Msg("interrupting execution of further stacks")
				cancel()

				if interruptions >= 3 {
					logger.Info().Msg("interrupted 3x times or more, killing child processes")
					kill()
					return
				}
			}
		}
	}()

	// map of stackName -> map of backendName -> outputs
	allOutputs := runutil.NewOnceMap[string, *run.OnceMap[string, cty.Value]]()

	err = sched.Run(func(run stackRun) error {
		errs := errors.L()

		failedTaskIndex := -1

	tasksLoop:
		for taskIndex, task := range run.Tasks {
			acquireResource()

			// For cloud sync, we always assume that there's a single task per stack.
			cloudRun := stackCloudRun{Stack: run.Stack, Task: task}

			select {
			case <-cancelCtx.Done():
				e.cloudSyncAfter(cloudRun, runResult{ExitCode: -1}, errors.E(ErrRunCanceled))
				releaseResource()
				continue tasksLoop
			default:
			}

			if !opts.Quiet && !opts.ScriptRun {
				printer.Stderr.Println(printPrefix + " Entering stack in " + run.Stack.String())
			}

			if !opts.Quiet && opts.ScriptRun {
				printScriptCommand(e.printers.Stdout, run.Stack, task)
			}

			logger := log.With().
				Stringer("stack", run.Stack).
				Bool("enable_sharing", task.EnableSharing).
				Bool("mock_on_fail", task.MockOnFail).
				Logger()

			cfg, _ := e.Config().Lookup(run.Stack.Dir)
			environ := newEnvironFrom(stackEnvs[run.Stack.Dir])

			if task.EnableSharing {
				for _, in := range cfg.Node.Inputs {
					evalctx, err := e.setupEvalContext(run.Stack, map[string]string{})
					if err != nil {
						errs.Append(errors.E(err, "failed to setup evaluation context"))
						e.cloudSyncAfter(cloudRun, runResult{ExitCode: -1}, errors.E(ErrRunCommandNotExecuted, err))
						releaseResource()
						failedTaskIndex = taskIndex
						if !continueOnError {
							cancel()
						}
						break tasksLoop
					}
					input, err := config.EvalInput(evalctx, in)
					if err != nil {
						errs.Append(errors.E(err, "failed to evaluate input block"))
						e.cloudSyncAfter(cloudRun, runResult{ExitCode: -1}, errors.E(ErrRunCommandNotExecuted, err))
						releaseResource()
						failedTaskIndex = taskIndex
						if !continueOnError {
							cancel()
						}
						break tasksLoop
					}
					otherStack, found, err := e.stackManager().StackByID(input.FromStackID)
					if err != nil {
						errs.Append(errors.E(err, "populating stack inputs from stack.id %s", input.FromStackID))
						e.cloudSyncAfter(cloudRun, runResult{ExitCode: -1}, errors.E(ErrRunCommandNotExecuted, err))
						releaseResource()
						failedTaskIndex = taskIndex
						if !continueOnError {
							cancel()
						}
						break tasksLoop
					}
					if !found {
						err := errors.E(
							"Stack %s needs output from stack ID %q but it cannot be found",
							run.Stack.Dir,
							input.FromStackID)

						errs.Append(err)

						e.cloudSyncAfter(cloudRun, runResult{ExitCode: -1}, errors.E(ErrRunCommandNotExecuted, err))
						releaseResource()
						failedTaskIndex = taskIndex
						if !continueOnError {
							cancel()
						}
						break tasksLoop
					}

					logger.Debug().Msgf("Stack depends on outputs from stack %s", otherStack.Dir)

					backend, ok := cfg.SharingBackend(input.Backend)
					if !ok {
						err := errors.E("backend %s not found", input.Backend)
						errs.Append(err)
						e.cloudSyncAfter(cloudRun, runResult{ExitCode: -1}, errors.E(ErrRunCommandNotExecuted, err))
						releaseResource()
						failedTaskIndex = taskIndex
						if !continueOnError {
							cancel()
						}
						break tasksLoop
					}

					stackOutputs, _ := allOutputs.GetOrInit(otherStack.Dir.String(), func() (*runutil.OnceMap[string, cty.Value], error) {
						return runutil.NewOnceMap[string, cty.Value](), nil
					})

					outputsVal, err := stackOutputs.GetOrInit(backend.Name, func() (cty.Value, error) {
						var stdout bytes.Buffer
						var stderr bytes.Buffer
						cmd := exec.Command(backend.Command[0], backend.Command[1:]...)
						cmd.Stdout = &stdout
						cmd.Stderr = &stderr
						cmd.Dir = otherStack.HostDir(e.Config())
						var inputVal cty.Value
						err := cmd.Run()
						if err != nil {
							if !task.MockOnFail {
								err := errors.E(err, "failed to execute: (cmd: %s) (stdout: %s) (stderr: %s)", cmd.String(), stdout.String(), stderr.String())
								errs.Append(err)
								e.cloudSyncAfter(cloudRun, runResult{ExitCode: -1}, errors.E(ErrRunCommandNotExecuted, err))
								releaseResource()
								failedTaskIndex = taskIndex
								if !continueOnError {
									cancel()
								}
								return cty.Value{}, err
							}

							printer.Stderr.WarnWithDetails(
								"failed to execute `sharing_backend` command",
								errors.E(err, "(cmd: %s) (stdout: %s) (stderr: %s)", cmd.String(), stdout.String(), stderr.String()),
							)
						} else {
							stdoutBytes := stdout.Bytes()
							typ, err := json.ImpliedType(stdoutBytes)
							if err != nil {
								err := errors.E(err, "unmashaling sharing_backend output")
								errs.Append(err)
								e.cloudSyncAfter(cloudRun, runResult{ExitCode: -1}, errors.E(ErrRunCommandNotExecuted, err))
								releaseResource()
								failedTaskIndex = taskIndex
								if !continueOnError {
									cancel()
								}
								return cty.Value{}, err

							}
							inputVal, err = json.Unmarshal(stdoutBytes, typ)
							if err != nil {
								err := errors.E(err, "unmashaling sharing_backend output")
								errs.Append(err)
								e.cloudSyncAfter(cloudRun, runResult{ExitCode: -1}, errors.E(ErrRunCommandNotExecuted, err))
								releaseResource()
								failedTaskIndex = taskIndex
								if !continueOnError {
									cancel()
								}
								return cty.Value{}, err
							}
						}
						return inputVal, nil
					})
					if err != nil {
						break tasksLoop
					}

					evalctx.SetNamespaceRaw("outputs", outputsVal)
					inputVal, inputErr := input.Value(evalctx)
					mockVal, mockFound, mockErr := input.Mock(evalctx)

					if inputErr != nil {
						if !task.MockOnFail || !mockFound || mockErr != nil {
							err := errors.E(inputErr, "evaluating input value")
							errs.Append(err)
							if mockErr != nil {
								errs.Append(errors.E(mockErr, "failed to evaluate input mock"))
							}
							e.cloudSyncAfter(cloudRun, runResult{ExitCode: -1}, errors.E(ErrRunCommandNotExecuted, err))
							releaseResource()
							failedTaskIndex = taskIndex
							if !continueOnError {
								cancel()
							}
							break tasksLoop
						}

						inputVal = mockVal
					}
					environ = append(environ, fmt.Sprintf("TF_VAR_%s=%s", input.Name, string(ast.TokensForValue(inputVal).Bytes())))
				}
			}

			cloudRun.Env = environ

			cmdStr := strings.Join(task.Cmd, " ")
			logger = logger.With().
				Str("cmd", cmdStr).
				Logger()

			cmdPath, err := runutil.LookPath(task.Cmd[0], environ)
			if err != nil {
				e.cloudSyncAfter(cloudRun, runResult{ExitCode: -1}, errors.E(ErrRunCommandNotExecuted, err))
				errs.Append(errors.E(err, "running `%s` in stack %s", cmdStr, run.Stack.Dir))
				releaseResource()
				failedTaskIndex = taskIndex
				if !continueOnError {
					cancel()
				}
				break tasksLoop
			}

			cmd := exec.Command(cmdPath, task.Cmd[1:]...)
			cmd.Dir = run.Stack.HostDir(e.Config())
			cmd.Env = environ

			stdin := opts.Stdin
			stdout := opts.Stdout
			stderr := opts.Stderr

			logSyncWait := func() {}
			if e.IsCloudEnabled() && (task.CloudSyncDeployment || task.CloudSyncPreview) {
				logSyncer := cloud.NewLogSyncer(func(logs cloud.CommandLogs) {
					e.syncLogs(&logger, run, logs)
				})
				stdout = logSyncer.NewBuffer(cloud.StdoutLogChannel, stdout)
				stderr = logSyncer.NewBuffer(cloud.StderrLogChannel, stderr)

				logSyncWait = logSyncer.Wait
			}

			cmd.Stdin = stdin
			cmd.Stdout = stdout
			cmd.Stderr = stderr

			e.cloudSyncBefore(cloudRun)

			if !opts.Quiet && !opts.ScriptRun {
				printer.Stderr.Println(printPrefix + " Executing command " + strconv.Quote(cmdStr))
			}

			if opts.DryRun {
				releaseResource()
				continue tasksLoop
			}

			startTime := time.Now().UTC()

			if err := cmd.Start(); err != nil {
				endTime := time.Now().UTC()

				logSyncWait()

				res := runResult{
					ExitCode:   -1,
					StartedAt:  &startTime,
					FinishedAt: &endTime,
				}
				e.cloudSyncAfter(cloudRun, res, errors.E(err, ErrRunFailed))
				errs.Append(errors.E(err, "running %s (at stack %s)", cmd, run.Stack.Dir))

				releaseResource()
				failedTaskIndex = taskIndex
				if !continueOnError {
					cancel()
				}
				break tasksLoop
			}

			resultc := makeResultChannel(cmd)

			select {
			case <-killCtx.Done():
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
				e.cloudSyncAfter(cloudRun, res, errors.E(ErrRunCanceled))
				errs.Append(errors.E(ErrRunCanceled, "execution aborted by CTRL-C (3x)"))
				releaseResource()
				failedTaskIndex = taskIndex
				if !continueOnError {
					cancel()
				}
				break tasksLoop

			case result := <-resultc:
				logSyncWait()

				var err error
				if !task.isSuccessExit(result.cmd.ProcessState.ExitCode()) {
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

				e.cloudSyncAfter(cloudRun, res, err)
				releaseResource()
				if err != nil {
					failedTaskIndex = taskIndex
					if !continueOnError {
						cancel()
					}
					break tasksLoop
				}
			}
		}

		if failedTaskIndex != -1 && run.SyncTaskIndex != -1 && failedTaskIndex < run.SyncTaskIndex {
			cloudRun := stackCloudRun{
				Stack: run.Stack,
				Task:  run.Tasks[run.SyncTaskIndex],
			}
			e.cloudSyncAfter(cloudRun, runResult{ExitCode: 1}, errors.E(ErrRunFailed))
		}

		return errs.AsError()
	})

	return err
}

func (e *Engine) syncLogs(logger *zerolog.Logger, run stackRun, logs cloud.CommandLogs) {
	data, _ := stdjson.Marshal(logs)
	logger.Debug().RawJSON("logs", data).Msg("synchronizing logs")
	ctx, cancel := context.WithTimeout(context.Background(), defaultCloudTimeout)
	defer cancel()
	stackID, _ := e.state.cloud.run.stackCloudID(run.Stack.ID)
	stackPreviewID, _ := e.state.cloud.run.cloudPreviewID(run.Stack.ID)
	err := e.state.cloud.client.SyncCommandLogs(
		ctx, e.state.cloud.run.orgUUID, stackID, e.state.cloud.run.runUUID, logs, stackPreviewID,
	)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to sync logs")
	}
}

func (t stackRunTask) isSuccessExit(exitCode int) bool {
	if exitCode == 0 {
		return true
	}
	if t.CloudSyncDriftStatus || (t.CloudSyncPreview && t.CloudPlanFile != "") {
		return exitCode == 2
	}
	return false
}

// printScriptCommand pretty prints the cmd and attaches a "prompt" style prefix to it
// for example:
// /somestack (script:0 job:0.0)> echo hello
func printScriptCommand(p *printer.Printer, stack *config.Stack, run stackRunTask) {
	p.Println(
		color.GreenString(fmt.Sprintf("%s (script:%d job:%d.%d)>",
			stack.Dir.String(),
			run.ScriptIdx, run.ScriptJobIdx, run.ScriptCmdIdx)),
	)
	p.Println(color.YellowString(strings.Join(run.Cmd, " ")))
}

type cmdResult struct {
	cmd        *exec.Cmd
	err        error
	finishedAt *time.Time
}

func makeResultChannel(cmd *exec.Cmd) <-chan cmdResult {
	resultc := make(chan cmdResult)
	go func() {
		err := cmd.Wait()
		endTime := time.Now().UTC()

		resultc <- cmdResult{
			cmd:        cmd,
			err:        err,
			finishedAt: &endTime,
		}
		close(resultc)
	}()
	return resultc
}

func newEnvironFrom(stackEnviron []string) []string {
	environ := make([]string, len(os.Environ()))
	copy(environ, os.Environ())
	environ = append(environ, stackEnviron...)
	return environ
}

func loadAllStackEnvs(root *config.Root, runs []stackRun) (map[project.Path]runutil.EnvVars, error) {
	errs := errors.L()
	stackEnvs := map[project.Path]runutil.EnvVars{}
	for _, run := range runs {
		env, err := runutil.LoadEnv(root, run.Stack)
		errs.Append(err)
		stackEnvs[run.Stack.Dir] = env
	}

	if errs.AsError() != nil {
		return nil, errs.AsError()
	}
	return stackEnvs, nil
}
