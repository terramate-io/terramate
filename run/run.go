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

// Run runs the command in the stack.
func (c Cmd) Run(root string, stack stack.S) error {
	cmd := exec.Command(c.Path, c.Args...)
	cmd.Dir = stack.AbsPath()
	cmd.Env = c.Environ
	cmd.Stdin = c.Stdin
	cmd.Stdout = c.Stdout
	cmd.Stderr = c.Stderr
	cmd.Env = c.Environ

	log.Info().
		Stringer("stack", stack).
		Str("cmd", c.String()).
		Msg("Running command in stack")

	return cmd.Run()
}
