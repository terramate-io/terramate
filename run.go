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
	"os"
	"os/exec"
	"path/filepath"

	"github.com/madlambda/spells/errutil"
	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/stack"
)

const ErrRunCycleDetected errutil.Error = "cycle detected in run order"

func Run(root string, stacks []stack.S, cmdSpec *exec.Cmd) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error getting current directory: %w", err)
	}

	for _, stack := range stacks {
		cmd := *cmdSpec

		stackdir, _ := project.ShowDir(root, wd, stack.Dir)
		fmt.Fprintf(cmd.Stdout, "[%s] running %s\n", stackdir, &cmd)
		cmd.Dir = filepath.Join(root, stack.Dir)
		err := cmd.Run()
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.Stdout, "\n")
	}

	return nil
}
