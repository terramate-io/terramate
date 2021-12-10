package terramate

import (
	"fmt"
	"os/exec"
	"sort"

	"github.com/madlambda/spells/errutil"
)

const ErrRunCycleDetected errutil.Error = "cycle detected in run order"

// RunOrder calculates the order of execution for the stacks list.
func RunOrder(stacks []Stack) ([]Stack, error) {
	stackset := map[string]Stack{} // indexed by stack dir
	orders := map[string][]Stack{} // indexed by stack dir

	for _, stack := range stacks {
		stackset[stack.Dir] = stack

		reversedOrder := []Stack{stack}
		visited := map[string]struct{}{}
		err := walkOrderList(stack, afterGet, func(s Stack) error {
			if _, ok := visited[s.Dir]; ok {
				return ErrRunCycleDetected
			}

			visited[s.Dir] = struct{}{}
			stackset[s.Dir] = s
			reversedOrder = append(reversedOrder, s)
			return nil
		})

		if err != nil {
			return nil, err
		}

		orders[stack.Dir] = reverse(reversedOrder)
	}

	groups := []stackOrder{}
	for stackdir, order := range orders {
		groups = append(groups, stackOrder{
			s:     stackset[stackdir],
			order: order,
		})
	}

	sort.Sort(sort.Reverse(byOrderSize(groups)))

	order := []Stack{}
	visited := map[string]struct{}{}

	// build computed order by skipping seen stacks.
	for _, k := range groups {
		stacks := orders[k.s.Dir]
		for _, stack := range stacks {
			if _, ok := visited[stack.Dir]; ok {
				continue
			}

			order = append(order, stack)
			visited[stack.Dir] = struct{}{}
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

// walkOrderList walks through all stack order entries recursively, calling do
// for each loaded stack entry. It calls get() to retrieve the order list.
func walkOrderList(stack Stack, get getter, do func(s Stack) error) error {
	orderDirs := get(stack)
	stacks, err := LoadStacks(stack.Dir, orderDirs...)
	if err != nil {
		return err
	}

	for _, s := range stacks {
		err := do(s)
		if err != nil {
			return err
		}

		err = walkOrderList(s, get, do)
		if err != nil {
			return fmt.Errorf("walking order list of %s: %w", s, err)
		}
	}

	return nil
}

func afterGet(s Stack) []string { return s.After }

func reverse(list []Stack) []Stack {
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}

	return list // unneeded but useful for renaming the slice
}

type getter func(s Stack) []string

type stackOrder struct {
	s     Stack
	order []Stack
}

type byOrderSize []stackOrder

func (x byOrderSize) Len() int { return len(x) }
func (x byOrderSize) Less(i, j int) bool {
	// if both orders have the same length, order lexicographically by the stack
	// directory string.
	if len(x[i].order) == len(x[j].order) {
		return x[i].s.Dir < x[j].s.Dir
	}
	return len(x[i].order) < len(x[j].order)
}
func (x byOrderSize) Swap(i, j int) { x[i], x[j] = x[j], x[i] }
