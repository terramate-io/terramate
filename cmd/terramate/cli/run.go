// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"context"
	"encoding/json"

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
	"github.com/terramate-io/terramate/printer"
	prj "github.com/terramate-io/terramate/project"
	runutil "github.com/terramate-io/terramate/run"
	"github.com/terramate-io/terramate/run/dag"
	"github.com/terramate-io/terramate/scheduler"
	"github.com/terramate-io/terramate/scheduler/resource"
	"github.com/terramate-io/terramate/stack"
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

// stackRun contains a list of tasks to be run per stack.
type stackRun struct {
	Stack *config.Stack
	Tasks []stackRunTask
}

// stackCloudRun is a stackRun, but with a single task, because the cloud API only supports
// a single command per stack for any operation (deploy, drift, preview).
type stackCloudRun struct {
	Stack *config.Stack
	Task  stackRunTask
}

// stackRunTask declares a stack run context.
type stackRunTask struct {
	Cmd []string

	ScriptIdx    int
	ScriptJobIdx int
	ScriptCmdIdx int

	CloudSyncDeployment        bool
	CloudSyncDriftStatus       bool
	CloudSyncPreview           bool
	CloudSyncLayer             preview.Layer
	CloudSyncTerraformPlanFile string
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
	if t.CloudSyncDriftStatus || (t.CloudSyncPreview && t.CloudSyncTerraformPlanFile != "") {
		return exitCode == 2
	}
	return false
}

func (c *cli) runOnStacks() {
	c.gitSafeguardDefaultBranchIsReachable()

	if len(c.parsedArgs.Run.Command) == 0 {
		fatal("run expects a cmd", nil)
	}

	c.checkOutdatedGeneratedCode()
	c.checkCloudSync()

	var stacks config.List[*config.SortableStack]
	if c.parsedArgs.Run.NoRecursive {
		st, found, err := config.TryLoadStack(c.cfg(), prj.PrjAbsPath(c.rootdir(), c.wd()))
		if err != nil {
			fatal("loading stack in current directory", err)
		}

		if !found {
			fatal("--no-recursive provided but no stack found in the current directory", nil)
		}

		stacks = append(stacks, st.Sortable())
	} else {
		var err error
		stacks, err = c.computeSelectedStacks(true, parseStatusFilter(c.parsedArgs.Run.CloudStatus))
		if err != nil {
			fatal("computing selected stacks", err)
		}
	}

	if c.parsedArgs.Run.CloudSyncDeployment && c.parsedArgs.Run.CloudSyncDriftStatus {
		fatal(sprintf("--cloud-sync-deployment conflicts with --cloud-sync-drift-status"), nil)
	}

	if c.parsedArgs.Run.CloudSyncPreview && (c.parsedArgs.Run.CloudSyncDeployment || c.parsedArgs.Run.CloudSyncDriftStatus) {
		fatal("cannot use --cloud-sync-preview with --cloud-sync-deployment or --cloud-sync-drift-status", nil)
	}

	if c.parsedArgs.Run.CloudSyncTerraformPlanFile == "" && c.parsedArgs.Run.CloudSyncPreview {
		fatal("--cloud-sync-preview requires --cloud-sync-terraform-plan-file", nil)
	}

	cloudSyncEnabled := c.parsedArgs.Run.CloudSyncDeployment || c.parsedArgs.Run.CloudSyncDriftStatus || c.parsedArgs.Run.CloudSyncPreview

	if c.parsedArgs.Run.CloudSyncTerraformPlanFile != "" && !cloudSyncEnabled {
		fatal("--cloud-sync-terraform-plan-file requires flags --cloud-sync-deployment or --cloud-sync-drift-status or --cloud-sync-preview", nil)
	}

	if c.parsedArgs.Run.CloudSyncDeployment || c.parsedArgs.Run.CloudSyncDriftStatus || c.parsedArgs.Run.CloudSyncPreview {
		if !c.prj.isRepo {
			fatal("cloud features requires a git repository", nil)
		}
		c.ensureAllStackHaveIDs(stacks)
		c.detectCloudMetadata()
	}

	var runs []stackRun
	var err error
	for _, st := range stacks {
		run := stackRun{
			Stack: st.Stack,
			Tasks: []stackRunTask{
				{
					Cmd:                        c.parsedArgs.Run.Command,
					CloudSyncDeployment:        c.parsedArgs.Run.CloudSyncDeployment,
					CloudSyncDriftStatus:       c.parsedArgs.Run.CloudSyncDriftStatus,
					CloudSyncPreview:           c.parsedArgs.Run.CloudSyncPreview,
					CloudSyncTerraformPlanFile: c.parsedArgs.Run.CloudSyncTerraformPlanFile,
					CloudSyncLayer:             c.parsedArgs.Run.CloudSyncLayer,
				},
			},
		}
		if c.parsedArgs.Run.Eval {
			run.Tasks[0].Cmd, err = c.evalRunArgs(run.Stack, run.Tasks[0].Cmd)
			if err != nil {
				fatal("unable to evaluate command", err)
			}
		}
		runs = append(runs, run)
	}

	if c.parsedArgs.Run.CloudSyncDeployment {
		// This will just select all runs, since the CloudSyncDeployment was set just above.
		// Still, it's convenient to re-use this function here.
		deployRuns := selectCloudStackTasks(runs, isDeploymentTask)
		c.createCloudDeployment(deployRuns)
	}

	if c.parsedArgs.Run.CloudSyncPreview && c.cloudEnabled() {
		// See comment above.
		previewRuns := selectCloudStackTasks(runs, isPreviewTask)
		c.cloud.run.stackPreviews = c.createCloudPreview(previewRuns)
	}

	err = c.runAll(runs, runAllOptions{
		Quiet:           c.parsedArgs.Quiet,
		DryRun:          c.parsedArgs.Run.DryRun,
		Reverse:         c.parsedArgs.Run.Reverse,
		ScriptRun:       false,
		ContinueOnError: c.parsedArgs.Run.ContinueOnError,
		Parallel:        c.parsedArgs.Run.Parallel.Value,
	})
	if err != nil {
		fatal("one or more commands failed", err)
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
			fatal(sprintf("cycle detected: %s", reason), err)
		} else {
			fatal("failed to plan execution", err)
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

	err = sched.Run(func(run stackRun) error {
		errs := errors.L()

		for _, task := range run.Tasks {
			// For cloud sync, we always assume that there's a single task per stack.
			cloudRun := stackCloudRun{Stack: run.Stack, Task: task}

			select {
			case <-cancelCtx.Done():
				c.cloudSyncAfter(cloudRun, runResult{ExitCode: -1}, errors.E(ErrRunCanceled))
				continue
			default:
			}

			cmdStr := strings.Join(task.Cmd, " ")
			logger := log.With().
				Str("cmd", cmdStr).
				Stringer("stack", run.Stack).
				Logger()

			if opts.ScriptRun && !c.parsedArgs.Quiet {
				printScriptCommand(c.stderr, run.Stack, task)
			}

			if !opts.Quiet && !opts.ScriptRun {
				printer.Stderr.Println(printPrefix + " Entering stack in " + run.Stack.String())
			}

			c.cloudSyncBefore(cloudRun)

			environ := newEnvironFrom(stackEnvs[run.Stack.Dir])
			cmdPath, err := runutil.LookPath(task.Cmd[0], environ)
			if err != nil {
				c.cloudSyncAfter(cloudRun, runResult{ExitCode: -1}, errors.E(ErrRunCommandNotFound, err))
				errs.Append(errors.E(err, "running `%s` in stack %s", cmdStr, run.Stack.Dir))
				if continueOnError {
					continue
				}

				cancel()
				return errs.AsError()
			}

			if !opts.Quiet && !opts.ScriptRun {
				printer.Stderr.Println(printPrefix + " Executing command " + strconv.Quote(cmdStr))
			}

			if opts.DryRun {
				continue
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

			acquireResource()

			startTime := time.Now().UTC()

			if err := cmd.Start(); err != nil {
				endTime := time.Now().UTC()

				releaseResource()
				logSyncWait()

				res := runResult{
					ExitCode:   -1,
					StartedAt:  &startTime,
					FinishedAt: &endTime,
				}
				c.cloudSyncAfter(cloudRun, res, errors.E(err, ErrRunFailed))
				errs.Append(errors.E(err, "running %s (at stack %s)", cmd, run.Stack.Dir))
				if continueOnError {
					continue
				}

				cancel()
				return errs.AsError()
			}

			resultc := makeResultChannel(cmd)

			select {
			case <-killCtx.Done():
				if err := cmd.Process.Kill(); err != nil {
					logger.Debug().Err(err).Msg("unable to send kill signal to child process")
				}

				endTime := time.Now().UTC()

				releaseResource()
				logSyncWait()

				res := runResult{
					ExitCode:   -1,
					StartedAt:  &startTime,
					FinishedAt: &endTime,
				}
				c.cloudSyncAfter(cloudRun, res, errors.E(ErrRunCanceled))
				return errors.E(ErrRunCanceled, "execution aborted by CTRL-C (3x)")

			case result := <-resultc:
				releaseResource()
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
			}

			err = errs.AsError()
			if err != nil && !continueOnError {
				logger.Info().Msg("interrupting execution of further stacks")

				cancel()
				return err
			}
		}

		return errs.AsError()
	})

	return err
}

func (c *cli) syncLogs(logger *zerolog.Logger, run stackRun, logs cloud.CommandLogs) {
	data, _ := json.Marshal(logs)
	logger.Debug().RawJSON("logs", data).Msg("synchronizing logs")
	ctx, cancel := context.WithTimeout(context.Background(), defaultCloudTimeout)
	defer cancel()
	stackID := c.cloud.run.meta2id[run.Stack.ID]
	stackPreviewID := c.cloud.run.stackPreviews[run.Stack.ID]
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

func (c *cli) createCloudPreview(runs []stackCloudRun) map[string]string {
	previewRuns := make([]cloud.RunContext, len(runs))
	for i, run := range runs {
		previewRuns[i] = cloud.RunContext{
			Stack: run.Stack,
			Cmd:   run.Task.Cmd,
		}
	}

	affectedStacksMap := map[string]*config.Stack{}
	for _, st := range c.getAffectedStacks() {
		affectedStacksMap[st.Stack.ID] = st.Stack
	}

	pullRequest := c.cloud.run.prFromGHAEvent
	if pullRequest == nil || pullRequest.GetUpdatedAt().IsZero() {
		printer.Stderr.Warn("unable to read pull_request details from GITHUB_EVENT_PATH")
		c.disableCloudFeatures(cloudError())
		return map[string]string{}
	}

	technology := "other"
	technologyLayer := "default"
	for _, run := range runs {
		if run.Task.CloudSyncTerraformPlanFile != "" {
			technology = "terraform"
		}
		if layer := run.Task.CloudSyncLayer; layer != "" {
			technologyLayer = sprintf("custom:%s", layer)
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
			UpdatedAt:       pullRequest.GetUpdatedAt().Unix(),
			Technology:      technology,
			TechnologyLayer: technologyLayer,
			Repository:      c.prj.prettyRepo(),
			DefaultBranch:   c.prj.gitcfg().DefaultBranch,
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
		report, err = mgr.ListChanged(c.baseRef())
		if err != nil {
			fatal("listing changed stacks", err)
		}

	} else {
		report, err = mgr.List()
		if err != nil {
			fatal("listing stacks", err)
		}
	}

	c.affectedStacks = report.Stacks
	return c.affectedStacks
}
