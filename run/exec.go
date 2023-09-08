// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package run

import (
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/project"
)

const (
	// ErrFailed represents the error when the execution fails, whatever the reason.
	ErrFailed errors.Kind = "execution failed"

	// ErrCommandNotFound represents the error when the command cannot be found
	// in the system.
	ErrCommandNotFound errors.Kind = "command not found"

	// ErrAborted represents the error when the execution was canceled.
	ErrAborted errors.Kind = "execution aborted"

	// ErrSkipped represents the error when a stack is skipped due to other
	// failed stacks.
	ErrSkipped errors.Kind = "execution skipped due to failed stacks"
)

// ExecContext declares an stack execution context.
type ExecContext struct {
	Stack *config.Stack
	Cmd   []string
}

// BeforeHook is the function signature for executing a function before each
// stack is executed.
type BeforeHook func(s *config.Stack, cmd string)

// AfterHook is the function signature for executing a function after each
// stack is executed.
type AfterHook func(s *config.Stack, exitCode int, err error)

// ExecAll will execute the list of RunStack definitions. A RunStack
// defines the stack and its command to be executed.
// During the execution of this function the default behavior
// for signal handling will be changed so we can wait for the child
// process to exit before exiting Terramate.
//
// If continue on error is true this function will continue to execute
// commands on stacks even in face of failures, returning an error.L with all errors.
// If continue on error is false it will return as soon as it finds an error,
// returning a list with a single error inside.
func ExecAll(
	root *config.Root,
	runStacks []ExecContext,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
	continueOnError bool,
	before BeforeHook,
	after AfterHook,
) error {
	logger := log.With().
		Str("action", "run.ExecAll()").
		Logger()

	const signalsBuffer = 10

	errs := errors.L()
	stackEnvs := map[project.Path]EnvVars{}

	logger.Trace().Msg("loading stacks run environment variables")
	for _, elem := range runStacks {
		env, err := LoadEnv(root, elem.Stack)
		errs.Append(err)
		stackEnvs[elem.Stack.Dir] = env
	}

	if errs.AsError() != nil {
		return errs.AsError()
	}

	logger.Trace().Msg("loaded stacks run environment variables, running commands")

	signals := make(chan os.Signal, signalsBuffer)
	signal.Notify(signals, os.Interrupt)
	defer signal.Reset(os.Interrupt)

	cmds := make(chan *exec.Cmd)
	defer close(cmds)

	skipStacks := func(stacks []ExecContext) {
		for _, run := range stacks {
			after(run.Stack, -1, errors.E(ErrSkipped))
		}
	}

	results := startCmdConsumer(cmds)

	for i, run := range runStacks {
		cmdStr := strings.Join(run.Cmd, " ")
		logger := log.With().
			Str("cmd", cmdStr).
			Stringer("stack", run.Stack).
			Logger()

		before(run.Stack, cmdStr)

		environ := make([]string, len(os.Environ()))
		copy(environ, os.Environ())
		environ = append(environ, stackEnvs[run.Stack.Dir]...)
		cmdPath, err := lookPath(run.Cmd[0], environ)
		if err != nil {
			after(run.Stack, -1, errors.E(err, ErrCommandNotFound))
			errs.Append(errors.E(err, "running `%s` in stack %s", cmdStr, run.Stack.Dir))
			if continueOnError {
				continue
			}
			skipStacks(runStacks[i+1:])
			return errs.AsError()
		}
		cmd := exec.Command(cmdPath, run.Cmd[1:]...)
		cmd.Dir = run.Stack.HostDir(root)
		cmd.Env = environ
		cmd.Stdin = stdin
		cmd.Stdout = stdout
		cmd.Stderr = stderr

		logger.Info().Msg("running")

		if err := cmd.Start(); err != nil {
			after(run.Stack, cmd.ProcessState.ExitCode(), errors.E(err, ErrFailed))
			errs.Append(errors.E(run.Stack, err, "running %s", cmd))
			if continueOnError {
				continue
			}
			skipStacks(runStacks[i+1:])
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
				}
			case result := <-results:
				logger.Trace().Msg("got command result")
				if result.err != nil {
					err := result.err
					if interruptions >= 3 {
						after(run.Stack, -1, errors.E(ErrAborted, err))
					} else {
						after(run.Stack, result.cmd.ProcessState.ExitCode(), errors.E(ErrFailed, err))
					}
					errs.Append(errors.E(err, "running %s (at stack %s)", result.cmd, run.Stack.Dir))
					if !continueOnError {
						skipStacks(runStacks[i+1:])
						return errs.AsError()
					}
				} else {
					after(run.Stack, 0, nil)
				}

				cmdIsRunning = false
			}
		}

		if interruptions > 0 {
			logger.Info().Msg("interrupting execution of further stacks")

			skipStacks(runStacks[i+1:])
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
