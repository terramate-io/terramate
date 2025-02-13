// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"bytes"
	"context"
	stdjson "encoding/json"
	"regexp"

	stdfmt "fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/preview"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/printer"
	prj "github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/run"

	runutil "github.com/terramate-io/terramate/run"
	"github.com/terramate-io/terramate/run/dag"
	"github.com/terramate-io/terramate/scheduler"
	"github.com/terramate-io/terramate/scheduler/resource"
	"github.com/terramate-io/terramate/stack"
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

	cloudSyncPreviewCICDWarning = "--sync-preview is only supported in GitHub Actions workflows, Gitlab CICD pipelines or Bitbucket Cloud Pipelines"
)

// stackRun contains a list of tasks to be run per stack.
type stackRun struct {
	Stack         *config.Stack
	Tasks         []stackRunTask
	SyncTaskIndex int // index of the task with sync options
}

// stackCloudRun is a stackRun, but with a single task, because the cloud API only supports
// a single command per stack for any operation (deploy, drift, preview).
type stackCloudRun struct {
	Target string
	Stack  *config.Stack
	Task   stackRunTask
	Env    []string
}

// stackRunTask declares a stack run context.
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

// runResult contains exit code and duration of a completed run.
type runResult struct {
	ExitCode   int
	StartedAt  *time.Time
	FinishedAt *time.Time
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

func selectPlanFile(terraformPlan, tofuPlan string) (planfile, provisioner string) {
	if tofuPlan != "" {
		planfile = tofuPlan
		provisioner = ProvisionerOpenTofu
	} else if terraformPlan != "" {
		planfile = terraformPlan
		provisioner = ProvisionerTerraform
	}
	return
}

func (c *cli) runOnStacks() {
	c.gitSafeguardDefaultBranchIsReachable()

	if len(c.parsedArgs.Run.Command) == 0 {
		fatal("run expects a cmd")
	}

	c.checkOutdatedGeneratedCode()
	c.checkCloudSync()

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
		var err error
		stackFilter := parseStatusFilter(c.parsedArgs.Run.Status)
		deploymentFilter := parseDeploymentStatusFilter(c.parsedArgs.Run.DeploymentStatus)
		driftFilter := parseDriftStatusFilter(c.parsedArgs.Run.DriftStatus)
		stacks, err = c.computeSelectedStacks(true, c.parsedArgs.Run.outputsSharingFlags, c.parsedArgs.Run.Target, cloud.StatusFilters{
			StackStatus:      stackFilter,
			DeploymentStatus: deploymentFilter,
			DriftStatus:      driftFilter,
		})
		if err != nil {
			fatalWithDetailf(err, "computing selected stacks")
		}
	}

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

	c.checkTargetsConfiguration(c.parsedArgs.Run.Target, c.parsedArgs.Run.FromTarget, func(isTargetSet bool) {
		isStatusSet := c.parsedArgs.Run.Status != ""
		isUsingCloudFeat := cloudSyncEnabled || isStatusSet

		if isTargetSet && !isUsingCloudFeat {
			fatal("--target must be used together with --sync-deployment, --sync-drift-status, --sync-preview, or --status")
		} else if !isTargetSet && isUsingCloudFeat {
			fatal("--sync-*/--status flags require --target when terramate.config.cloud.targets.enabled is true")
		}
	})

	if c.parsedArgs.Run.FromTarget != "" && !cloudSyncEnabled {
		fatal("--from-target must be used together with --sync-deployment, --sync-drift-status, or --sync-preview")
	}

	if cloudSyncEnabled {
		if !c.prj.isRepo {
			fatal("cloud features requires a git repository")
		}
		c.ensureAllStackHaveIDs(stacks)
		c.detectCloudMetadata()
	}

	isCICD := os.Getenv("GITHUB_ACTIONS") != "" || os.Getenv("GITLAB_CI") != "" || os.Getenv("BITBUCKET_BUILD_NUMBER") != ""
	if c.parsedArgs.Run.SyncPreview && !isCICD {
		printer.Stderr.Warn(cloudSyncPreviewCICDWarning)
		c.disableCloudFeatures(errors.E(cloudSyncPreviewCICDWarning))
	}

	var runs []stackRun
	var err error
	for _, st := range stacks {
		run := stackRun{
			SyncTaskIndex: -1,
			Stack:         st.Stack,
			Tasks: []stackRunTask{
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

	if c.parsedArgs.Run.SyncDeployment {
		// This will just select all runs, since the CloudSyncDeployment was set just above.
		// Still, it's convenient to re-use this function here.
		deployRuns := selectCloudStackTasks(runs, isDeploymentTask)
		c.createCloudDeployment(deployRuns)
	}

	if c.parsedArgs.Run.SyncPreview && c.cloudEnabled() {
		// See comment above.
		previewRuns := selectCloudStackTasks(runs, isPreviewTask)
		for metaID, previewID := range c.createCloudPreview(previewRuns, c.parsedArgs.Run.Target, c.parsedArgs.Run.FromTarget) {
			c.cloud.run.setMeta2PreviewID(metaID, previewID)
		}
	}

	err = c.runAll(runs, runAllOptions{
		Quiet:           c.parsedArgs.Quiet,
		DryRun:          c.parsedArgs.Run.DryRun,
		Reverse:         c.parsedArgs.Run.Reverse,
		ScriptRun:       false,
		ContinueOnError: c.parsedArgs.Run.ContinueOnError,
		Parallel:        c.parsedArgs.Run.Parallel,
	})
	if err != nil {
		fatalWithDetailf(err, "one or more commands failed")
	}
}

// runAllOptions define named flags for runAll
type runAllOptions struct {
	Quiet           bool
	DryRun          bool
	Reverse         bool
	ScriptRun       bool
	ContinueOnError bool
	Parallel        int
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
	runs []stackRun,
	opts runAllOptions,
) error {
	// Construct a DAG from the list of stackRuns, based on the implicit and
	// explicit dependencies between stacks.
	d, reason, err := runutil.BuildDAGFromStacks(c.cfg(), runs,
		func(run stackRun) *config.Stack { return run.Stack })
	if err != nil {
		if errors.IsKind(err, dag.ErrCycleDetected) {
			fatalWithDetailf(err, "cycle detected: %s", reason)
		} else {
			fatalWithDetailf(err, "failed to plan execution")
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
	stackEnvs, err := c.loadAllStackEnvs(runs)
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
		printPrefix = stdfmt.Sprintf("%s (dry-run)", printPrefix)
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
				c.cloudSyncAfter(cloudRun, runResult{ExitCode: -1}, errors.E(ErrRunCanceled))
				releaseResource()
				continue tasksLoop
			default:
			}

			if !opts.Quiet && !opts.ScriptRun {
				printer.Stderr.Println(printPrefix + " Entering stack in " + run.Stack.String())
			}

			if !opts.Quiet && opts.ScriptRun {
				printScriptCommand(c.stderr, run.Stack, task)
			}

			logger := log.With().
				Stringer("stack", run.Stack).
				Bool("enable_sharing", task.EnableSharing).
				Bool("mock_on_fail", task.MockOnFail).
				Logger()

			cfg, _ := c.cfg().Lookup(run.Stack.Dir)
			environ := newEnvironFrom(stackEnvs[run.Stack.Dir])

			if task.EnableSharing {
				for _, in := range cfg.Node.Inputs {
					evalctx := c.setupEvalContext(run.Stack, map[string]string{})
					input, err := config.EvalInput(evalctx, in)
					if err != nil {
						errs.Append(errors.E(err, "failed to evaluate input block"))
						c.cloudSyncAfter(cloudRun, runResult{ExitCode: -1}, errors.E(ErrRunCommandNotExecuted, err))
						releaseResource()
						failedTaskIndex = taskIndex
						if !continueOnError {
							cancel()
						}
						break tasksLoop
					}
					otherStack, found, err := c.stackManager().StackByID(input.FromStackID)
					if err != nil {
						errs.Append(errors.E(err, "populating stack inputs from stack.id %s", input.FromStackID))
						c.cloudSyncAfter(cloudRun, runResult{ExitCode: -1}, errors.E(ErrRunCommandNotExecuted, err))
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

						c.cloudSyncAfter(cloudRun, runResult{ExitCode: -1}, errors.E(ErrRunCommandNotExecuted, err))
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
						c.cloudSyncAfter(cloudRun, runResult{ExitCode: -1}, errors.E(ErrRunCommandNotExecuted, err))
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
						cmd.Dir = otherStack.HostDir(c.cfg())
						var inputVal cty.Value
						err := cmd.Run()
						if err != nil {
							if !task.MockOnFail {
								err := errors.E(err, "failed to execute: (cmd: %s) (stdout: %s) (stderr: %s)", cmd.String(), stdout.String(), stderr.String())
								errs.Append(err)
								c.cloudSyncAfter(cloudRun, runResult{ExitCode: -1}, errors.E(ErrRunCommandNotExecuted, err))
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
								c.cloudSyncAfter(cloudRun, runResult{ExitCode: -1}, errors.E(ErrRunCommandNotExecuted, err))
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
								c.cloudSyncAfter(cloudRun, runResult{ExitCode: -1}, errors.E(ErrRunCommandNotExecuted, err))
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
							c.cloudSyncAfter(cloudRun, runResult{ExitCode: -1}, errors.E(ErrRunCommandNotExecuted, err))
							releaseResource()
							failedTaskIndex = taskIndex
							if !continueOnError {
								cancel()
							}
							break tasksLoop
						}

						inputVal = mockVal
					}
					environ = append(environ, stdfmt.Sprintf("TF_VAR_%s=%s", input.Name, string(ast.TokensForValue(inputVal).Bytes())))
				}
			}

			cloudRun.Env = environ

			cmdStr := strings.Join(task.Cmd, " ")
			logger = logger.With().
				Str("cmd", cmdStr).
				Logger()

			cmdPath, err := runutil.LookPath(task.Cmd[0], environ)
			if err != nil {
				c.cloudSyncAfter(cloudRun, runResult{ExitCode: -1}, errors.E(ErrRunCommandNotExecuted, err))
				errs.Append(errors.E(err, "running `%s` in stack %s", cmdStr, run.Stack.Dir))
				releaseResource()
				failedTaskIndex = taskIndex
				if !continueOnError {
					cancel()
				}
				break tasksLoop
			}

			cmd := exec.Command(cmdPath, task.Cmd[1:]...)
			cmd.Dir = run.Stack.HostDir(c.cfg())
			cmd.Env = environ

			stdout := c.stdout
			stderr := c.stderr

			logSyncWait := func() {}
			if c.cloudEnabled() && (task.CloudSyncDeployment || task.CloudSyncPreview) {
				logSyncer := cloud.NewLogSyncer(func(logs cloud.CommandLogs) {
					c.syncLogs(&logger, run, logs)
				})
				stdout = logSyncer.NewBuffer(cloud.StdoutLogChannel, c.stdout)
				stderr = logSyncer.NewBuffer(cloud.StderrLogChannel, c.stderr)

				logSyncWait = logSyncer.Wait
			}

			cmd.Stdin = c.stdin
			cmd.Stdout = stdout
			cmd.Stderr = stderr

			c.cloudSyncBefore(cloudRun)

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
				c.cloudSyncAfter(cloudRun, res, errors.E(err, ErrRunFailed))
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
				c.cloudSyncAfter(cloudRun, res, errors.E(ErrRunCanceled))
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

				c.cloudSyncAfter(cloudRun, res, err)
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
			c.cloudSyncAfter(cloudRun, runResult{ExitCode: 1}, errors.E(ErrRunFailed))
		}

		return errs.AsError()
	})

	return err
}

func (c *cli) syncLogs(logger *zerolog.Logger, run stackRun, logs cloud.CommandLogs) {
	data, _ := stdjson.Marshal(logs)
	logger.Debug().RawJSON("logs", data).Msg("synchronizing logs")
	ctx, cancel := context.WithTimeout(context.Background(), defaultCloudTimeout)
	defer cancel()
	stackID, _ := c.cloud.run.stackCloudID(run.Stack.ID)
	stackPreviewID, _ := c.cloud.run.cloudPreviewID(run.Stack.ID)
	err := c.cloud.client.SyncCommandLogs(
		ctx, c.cloud.run.orgUUID, stackID, c.cloud.run.runUUID, logs, stackPreviewID,
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

func (c *cli) loadAllStackEnvs(runs []stackRun) (map[prj.Path]runutil.EnvVars, error) {
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

func (c *cli) createCloudPreview(runs []stackCloudRun, target, fromTarget string) map[string]string {
	previewRuns := make([]cloud.RunContext, len(runs))
	for i, run := range runs {
		previewRuns[i] = cloud.RunContext{
			StackID: run.Stack.ID,
			Cmd:     run.Task.Cmd,
		}
	}

	affectedStacksMap := map[string]cloud.Stack{}
	for _, st := range c.getAffectedStacks() {
		affectedStacksMap[st.Stack.ID] = cloud.Stack{
			Path:            st.Stack.Dir.String(),
			MetaID:          strings.ToLower(st.Stack.ID),
			MetaName:        st.Stack.Name,
			MetaDescription: st.Stack.Description,
			MetaTags:        st.Stack.Tags,
			Repository:      c.prj.prettyRepo(),
			Target:          target,
			FromTarget:      fromTarget,
			DefaultBranch:   c.prj.gitcfg().DefaultBranch,
		}
	}

	if c.cloud.run.reviewRequest == nil || c.cloud.run.rrEvent.pushedAt == nil {
		printer.Stderr.WarnWithDetails(
			"unable to create preview: missing review request information",
			errors.E("--sync-preview can only be used when GITHUB_TOKEN or GITLAB_TOKEN is exported and Terramate runs in a CI/CD environment triggered by a Pull/Merge Request event"),
		)
		c.disableCloudFeatures(cloudError())
		return map[string]string{}
	}

	c.cloud.run.reviewRequest.PushedAt = c.cloud.run.rrEvent.pushedAt

	// preview always requires a commit_sha, so if the API failed to provide it, we should give the HEAD commit.
	if c.cloud.run.rrEvent.commitSHA == "" {
		c.cloud.run.rrEvent.commitSHA = c.prj.headCommit()
	}

	technology := "other"
	technologyLayer := "default"
	for _, run := range runs {
		if run.Task.CloudPlanFile != "" {
			technology = run.Task.CloudPlanProvisioner
		}
		if layer := run.Task.CloudSyncLayer; layer != "" {
			technologyLayer = stdfmt.Sprintf("custom:%s", layer)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultCloudTimeout)
	defer cancel()
	createdPreview, err := c.cloud.client.CreatePreview(
		ctx,
		cloud.CreatePreviewOpts{
			Runs:            previewRuns,
			AffectedStacks:  affectedStacksMap,
			OrgUUID:         c.cloud.run.orgUUID,
			PushedAt:        *c.cloud.run.rrEvent.pushedAt,
			CommitSHA:       c.cloud.run.rrEvent.commitSHA,
			Technology:      technology,
			TechnologyLayer: technologyLayer,
			ReviewRequest:   c.cloud.run.reviewRequest,
			Metadata:        c.cloud.run.metadata,
		},
	)
	if err != nil {
		printer.Stderr.WarnWithDetails("unable to create preview", err)
		c.disableCloudFeatures(cloudError())
		return map[string]string{}
	}

	printer.Stderr.Success(stdfmt.Sprintf("Preview created (id: %s)", createdPreview.ID))

	if c.parsedArgs.Run.DebugPreviewURL != "" {
		c.writePreviewURL()
	}

	return createdPreview.StackPreviewsByMetaID
}

func (c *cli) writePreviewURL() {
	rrNumber := 0
	if c.cloud.run.metadata != nil && c.cloud.run.metadata.GithubPullRequestNumber != 0 {
		ctx, cancel := context.WithTimeout(context.Background(), defaultCloudTimeout)
		defer cancel()
		reviews, err := c.cloud.client.ListReviewRequests(ctx, c.cloud.run.orgUUID)
		if err != nil {
			printer.Stderr.Warn(stdfmt.Sprintf("unable to list review requests: %v", err))
			return
		}
		for _, review := range reviews {
			if review.Number == c.cloud.run.metadata.GithubPullRequestNumber &&
				review.CommitSHA == c.prj.headCommit() {
				rrNumber = int(review.ID)
			}
		}
	}

	cloudURL := "https://cloud.terramate.io"
	if c.cloud.client.BaseURL == "https://api.stg.terramate.io" {
		cloudURL = "https://cloud.stg.terramate.io"
	}

	var url = stdfmt.Sprintf("%s/o/%s/review-requests\n", cloudURL, c.cloud.run.orgName)
	if rrNumber != 0 {
		url = stdfmt.Sprintf("%s/o/%s/review-requests/%d\n",
			cloudURL,
			c.cloud.run.orgName,
			rrNumber)
	}

	err := os.WriteFile(c.parsedArgs.Run.DebugPreviewURL, []byte(url), 0644)
	if err != nil {
		printer.Stderr.Warn(stdfmt.Sprintf("unable to write preview URL to file: %v", err))
	}
}

// getAffectedStacks returns the list of stacks affected by the current command.
// c.affectedStacks is expected to be already set, if not it will be computed
// and cached.
func (c *cli) getAffectedStacks() []stack.Entry {
	if c.affectedStacks != nil {
		return c.affectedStacks
	}

	mgr := c.stackManager()

	var report *stack.Report
	var err error
	if c.parsedArgs.Changed {
		report, err = mgr.ListChanged(stack.ChangeConfig{
			BaseRef:            c.baseRef(),
			UntrackedChanges:   c.changeDetection.untracked,
			UncommittedChanges: c.changeDetection.uncommitted,
		})
		if err != nil {
			fatalWithDetailf(err, "listing changed stacks")
		}

	} else {
		report, err = mgr.List(true)
		if err != nil {
			fatalWithDetailf(err, "listing stacks")
		}
	}

	c.affectedStacks = report.Stacks
	return c.affectedStacks
}

const targetIDRegexPattern = "^[a-z0-9][-_a-z0-9]*[a-z0-9]$"

var targetIDRegex = regexp.MustCompile(targetIDRegexPattern)

func (c *cli) checkTargetsConfiguration(targetArg, fromTargetArg string, cloudCheckFn func(bool)) {
	isTargetSet := targetArg != ""
	isFromTargetSet := fromTargetArg != ""
	isTargetsEnabled := c.cfg().HasExperiment("targets") && c.cfg().IsTargetsEnabled()

	if isTargetSet {
		if !isTargetsEnabled {
			printer.Stderr.Error(`The "targets" feature is not enabled`)
			printer.Stderr.Println(`In order to enable it you must set the terramate.config.experiments attribute and set terramate.config.cloud.targets.enabled to true.`)
			printer.Stderr.Println(`Example:
	
terramate {
  config {
    experiments = ["targets"]
    cloud {
      targets {
        enabled = true
      }
    }
  }
}`)
			os.Exit(1)
		}

		// Here we should check if any cloud parameter is enabled for target to make sense.
		// The error messages should be different per caller.
		cloudCheckFn(true)

	} else {
		if isTargetsEnabled {
			// Here we should check if any cloud parameter is enabled that would require target.
			// The error messages should be different per caller.
			cloudCheckFn(false)
		}
	}

	if isFromTargetSet && !isTargetSet {
		fatal("--from-target requires --target")
	}

	if isTargetSet && !targetIDRegex.MatchString(targetArg) {
		fatalf("--target value has invalid format, it must match %q", targetIDRegexPattern)
	}

	if isFromTargetSet && !targetIDRegex.MatchString(fromTargetArg) {
		fatalf("--from-target value has invalid format, it must match %q", targetIDRegexPattern)
	}

	c.cloud.run.target = targetArg
}
