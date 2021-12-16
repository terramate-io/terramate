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
	"fmt"
	"os/exec"

	"github.com/madlambda/spells/errutil"
	"github.com/mineiros-io/terramate/stack"
)

const ErrRunCycleDetected errutil.Error = "cycle detected in run order"

func Run(stacks []stack.S, cmdSpec *exec.Cmd) error {
	for _, stack := range stacks {
		cmd := *cmdSpec

		fmt.Fprintf(cmd.Stdout, "[%s] running %s\n", stack.Dir, &cmd)
		cmd.Dir = stack.Dir
		err := cmd.Run()
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.Stdout, "\n")
	}

	return nil
}
