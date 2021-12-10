package terramate

import (
	"fmt"
	"os/exec"
	"sort"

	"github.com/madlambda/spells/errutil"
)

type orderGetter func(s Stack) []string

const ErrRunCycleDetected errutil.Error = "cycle detected in run order"

func RunOrder(stacks []Stack) ([]Stack, error) {
	orders := map[string][]Stack{}

	for _, stack := range stacks {
		after, err := LoadStacks(stack.Dir, stack.After...)
		if err != nil {
			return nil, err
		}

		visited := map[string]struct{}{}
		err = checkRunOrder(stack, after, afterGet, visited)
		if err != nil {
			return nil, err
		}

		order := []Stack{}
		err = walkOrder(stack, afterGet, func(s Stack) {
			order = append(order, s)
		})
		if err != nil {
			return nil, err
		}

		order = append(order, stack)
		orders[stack.Dir] = order
	}

	order := []Stack{}
	executed := map[string]struct{}{}

	keys := []string{}
	for k := range orders {
		keys = append(keys, k)
	}

	sort.StringSlice(keys).Sort()
	for _, k := range keys {
		stacks := orders[k]
		for _, stack := range stacks {
			if _, ok := executed[stack.Dir]; ok {
				continue
			}

			order = append(order, stack)
			executed[stack.Dir] = struct{}{}
		}
	}

	return order, nil
}

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

func checkRunOrder(stack Stack,
	deps []Stack,
	get orderGetter,
	visited map[string]struct{},
) error {
	visited[stack.Dir] = struct{}{}

	for _, b := range deps {
		if _, ok := visited[b.Dir]; ok {
			return ErrRunCycleDetected
		}

		children := get(b)
		stacks, err := LoadStacks(b.Dir, children...)
		if err != nil {
			return err
		}

		err = checkRunOrder(b, stacks, get, visited)
		if err != nil {
			return err
		}
	}

	return nil
}

func walkOrder(stack Stack, getOrder orderGetter, do func(s Stack)) error {
	dirs := getOrder(stack)
	stacks, err := LoadStacks(stack.Dir, dirs...)
	if err != nil {
		return err
	}

	for _, s := range stacks {
		do(s)

		err := walkOrder(s, getOrder, do)
		if err != nil {
			return fmt.Errorf("walking order list of %s: %w", s, err)
		}
	}

	return nil
}

func afterGet(s Stack) []string  { return s.After }
func beforeGet(s Stack) []string { return s.Before }
