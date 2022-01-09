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

package terramate

import (
	"os/exec"
	"path/filepath"

	"github.com/mineiros-io/terramate/stack"
	"github.com/rs/zerolog/log"
)

func Run(root string, stacks []stack.S, cmdSpec *exec.Cmd) error {
	for _, stack := range stacks {
		cmd := *cmdSpec

		log.Info().
			Str("stack", stack.Dir).
			Str("cmd", cmd.String()).
			Msg("Running command in stack")

		cmd.Dir = filepath.Join(root, stack.Dir)

		err := cmd.Run()

		if err != nil {
			return err
		}
	}

	return nil
}
