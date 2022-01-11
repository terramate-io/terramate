// Copyright 2021 Mineiros GmbH
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
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mineiros-io/terramate/stack"
	"github.com/rs/zerolog/log"
)

// Cmd describes a command to be executed by terramate.
type Cmd struct {
	Path   string    // Path is the command path.
	Args   []string  // Args is the command arguments.
	Stdin  io.Reader // Stdin is the process standard input.
	Stdout io.Writer // Stdout is the process standard output.
	Stderr io.Writer // Stderr is the process standard error.

	// Environ is the list of environment variables to be passed over to the cmd.
	Environ []string
}

func (c *Cmd) String() string {
	return c.Path + " " + strings.Join(c.Args, " ")
}

// Run runs the given cmdSpec in each stack.
func Run(root string, stacks []stack.S, cmdSpec Cmd) error {
	for _, stack := range stacks {
		cmd := exec.Command(cmdSpec.Path, cmdSpec.Args...)
		cmd.Dir = filepath.Join(root, stack.Dir)
		cmd.Env = cmdSpec.Environ
		cmd.Stdin = cmdSpec.Stdin
		cmd.Stdout = cmdSpec.Stdout
		cmd.Stderr = cmdSpec.Stderr
		cmd.Env = cmdSpec.Environ

		log.Info().
			Str("stack", stack.Dir).
			Str("cmd", cmdSpec.String()).
			Msg("Running command in stack")

		err := cmd.Run()
		if err != nil {
			return err
		}
	}

	return nil
}
