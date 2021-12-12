package terramate

import (
	"fmt"
	"os/exec"

	"github.com/madlambda/spells/errutil"
)

const ErrRunCycleDetected errutil.Error = "cycle detected in run order"

func Run(stacks []Stack, cmd *exec.Cmd) error {
	order, err := RunOrder(stacks)
	if err != nil {
		return err
	}

	for _, stack := range order {
		fmt.Fprintf(cmd.Stdout, "[%s] running %s\n", stack.Dir, cmd)
		cmd.Dir = stack.Dir
		err := cmd.Run()
		if err != nil {
			return err
		}

	}

	return nil
}
