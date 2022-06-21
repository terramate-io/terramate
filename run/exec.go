// Copyright 2022 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package run

import (
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/stack"
	"github.com/rs/zerolog/log"
)

// Exec will execute the given command on the given stack list
// During the execution of this function the default behavior
// for signal handling will be changed. The behavior implemented is:
//
// - 1 x CTRL-C -> graceful shutdown, current stack executes but no other stack is executed.
//   No signal is forwarded to the sub process.
// - 2 x CTRL-C -> forward CTRL-C to running sub process
// - 3 x CTRL-C -> forward CTRL-C to running sub process 2nd time
// - 4 x CTRL-C -> kill running sub process with SIGKILL
//
// If continue on error is true this function will continue to execute
// commands on stacks even in face of failures, returning an error.L with all errors.
// If continue on error is false it will return as soon as it finds an error,
// returning a list with a single error inside.
func Exec(
	stacks []stack.S,
	cmd []string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
	continueOnError bool,
) error {
	logger := log.With().
		Str("action", "run.Exec()").
		Str("cmd", strings.Join(cmd, " ")).
		Logger()

	// Should be at least 1 to avoid losing signals
	// We are using 4 since it is the number of interrupts
	// that we handle to do a hard kill, which we could receive
	// before starting to run a command.
	const signalsBuffer = 4

	signals := make(chan os.Signal, signalsBuffer)
	signal.Notify(signals)
	defer signal.Reset()

	cmds := make(chan *exec.Cmd)
	defer close(cmds)

	results := startCmdRunner(cmds)

	errs := errors.L()

	for _, stack := range stacks {
		cmd := exec.Command(cmd[0], cmd[1:]...)
		cmd.Dir = stack.HostPath()
		cmd.Env = os.Environ()
		cmd.Stdin = stdin
		cmd.Stdout = stdout
		cmd.Stderr = stderr

		logger := logger.With().
			Stringer("stack", stack).
			Logger()

		logger.Info().Msg("Running")

		// The child process should not get signals directly since
		// we want to handle first interrupt differently.
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

		if err := cmd.Start(); err != nil {
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
				logger.Trace().
					Str("signal", sig.String()).
					Msg("received signal")

				if sig.String() != os.Interrupt.String() {
					logger.Trace().Msg("not a interruption signal, relaying")

					if err := cmd.Process.Signal(sig); err != nil {
						logger.Debug().Err(err).Msg("unable to send signal to child process")
					}
					break
				}

				logger.Trace().Msg("interruption signal, handling")

				interruptions++
				switch interruptions {
				case 1:
					logger.Info().Msg("interruption, no more stacks will be run")
				case 2, 3:
					logger.Info().Msg("interrupted more than once, sending signal to child process")

					// TODO(katcipis): Sending interrupt signals will fail on windows.
					// Windows is not supported for now.
					if err := cmd.Process.Signal(sig); err != nil {
						logger.Debug().Err(err).Msg("unable to send signal to child process")
					}
				case 4:
					logger.Info().Msg("interrupted 4x times, killing child process")

					if err := cmd.Process.Kill(); err != nil {
						logger.Debug().Err(err).Msg("unable to send kill signal to child process")
					}
				}
			case err := <-results:
				logger.Trace().Msg("got command result")
				if err != nil {
					errs.Append(errors.E(stack, err, "running %s", cmd))
					if !continueOnError {
						return errs.AsError()
					}
				}
				cmdIsRunning = false
			}
		}

		if interruptions > 0 {
			logger.Trace().Msg("interrupting execution of further stacks")
			return errs.AsError()
		}
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
