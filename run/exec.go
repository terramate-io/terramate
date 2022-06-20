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
	"strings"

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

		err := cmd.Run()
		if err != nil {
			errs.Append(errors.E(stack, err, "running %s", cmd))
			if !continueOnError {
				return errs.AsError()
			}
		}
	}

	return errs.AsError()
}
