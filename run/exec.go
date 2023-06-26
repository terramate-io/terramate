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
	// ErrCanceled represents the error when the execution was canceled.
	ErrCanceled errors.Kind = "execution canceled"
)

// Exec will execute the given command on the given stack list
// During the execution of this function the default behavior
// for signal handling will be changed so we can wait for the child
// process to exit before exiting Terramate.
//
// If continue on error is true this function will continue to execute
// commands on stacks even in face of failures, returning an error.L with all errors.
// If continue on error is false it will return as soon as it finds an error,
// returning a list with a single error inside.
func Exec(
	root *config.Root,
	stacks config.List[*config.SortableStack],
	cmd []string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
	continueOnError bool,
	before func(s *config.Stack, cmd string),
	after func(s *config.Stack, err error),
) error {
	logger := log.With().
		Str("action", "run.Exec()").
		Str("cmd", strings.Join(cmd, " ")).
		Logger()

	const signalsBuffer = 10

	errs := errors.L()
	stackEnvs := map[project.Path]EnvVars{}

	logger.Trace().Msg("loading stacks run environment variables")
	for _, elem := range stacks {
		env, err := LoadEnv(root, elem.Stack)
		errs.Append(err)
		stackEnvs[elem.Dir()] = env
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

	results := startCmdRunner(cmds)

	for i, stack := range stacks {
		logger := log.With().
			Str("cmd", strings.Join(cmd, " ")).
			Stringer("stack", stack).
			Logger()

		cmd := exec.Command(cmd[0], cmd[1:]...)
		cmd.Dir = stack.HostDir(root)
		cmd.Env = append(os.Environ(), stackEnvs[stack.Dir()]...)
		cmd.Stdin = stdin
		cmd.Stdout = stdout
		cmd.Stderr = stderr

		logger.Info().Msg("running")

		before(stack.Stack, cmd.String())

		if err := cmd.Start(); err != nil {
			after(stack.Stack, errors.E(err, ErrFailed))
			errs.Append(errors.E(stack, err, "running %s", cmd))
			if continueOnError {
				continue
			}
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
			case err := <-results:
				logger.Trace().Msg("got command result")
				if err != nil {
					if interruptions >= 3 {
						after(stack.Stack, errors.E(ErrCanceled, err))
					} else {
						after(stack.Stack, errors.E(ErrFailed, err))
					}
					errs.Append(errors.E(err, "running %s (at stack %s)", cmd, stack.Dir()))
					if !continueOnError {
						return errs.AsError()
					}
				}
				cmdIsRunning = false
			}
		}

		if interruptions > 0 {
			logger.Info().Msg("interrupting execution of further stacks")

			index := i + 1
			for ; index < len(stacks); index++ {
				after(stacks[index].Stack, errors.E(ErrCanceled))
			}
			return errs.AsError()
		}

		after(stack.Stack, nil)
	}

	return errs.AsError()
}

func startCmdRunner(cmds <-chan *exec.Cmd) <-chan error {
	errs := make(chan error)
	go func() {
		for cmd := range cmds {
			errs <- cmd.Wait()
		}
		close(errs)
	}()
	return errs
}
