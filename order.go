package terramate

import (
	"fmt"
	"sort"
)

type OrderTree struct {
	Stack Stack
	After []OrderTree

	Cycle bool
}

func BuildOrderTree(stack Stack) (OrderTree, error) {
	return buildOrderTree(stack, map[string]struct{}{})
}

func RunOrder(stacks []Stack) ([]Stack, error) {
	trees := map[string]OrderTree{} // indexed by stackdir
	for _, stack := range stacks {
		tree, err := BuildOrderTree(stack)
		if err != nil {
			return nil, err
		}

		err = CheckCycle(tree)
		if err != nil {
			return nil, err
		}

		trees[stack.Dir] = tree
	}

	removeKeys := []string{}
	for key1, tree1 := range trees {
		for key2, tree2 := range trees {
			if key1 == key2 {
				continue
			}

			if IsSubtree(tree1, tree2) {
				removeKeys = append(removeKeys, key1)
			}
		}
	}

	for _, k := range removeKeys {
		delete(trees, k)
	}

	keys := []string{}
	for k := range trees {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	order := []Stack{}
	visited := map[string]struct{}{}
	for _, k := range keys {
		tree := trees[k]
		walkOrderTree(tree, func(s Stack) {
			if _, ok := visited[s.Dir]; !ok {
				order = append(order, s)
				visited[s.Dir] = struct{}{}
			}
		})
	}

	return order, nil
}

func walkOrderTree(tree OrderTree, do func(s Stack)) {
	for _, child := range tree.After {
		walkOrderTree(child, do)
	}

	do(tree.Stack)
}

func IsSubtree(t1, t2 OrderTree) bool {
	if t1.Stack.Dir == t2.Stack.Dir {
		return true
	}
	for _, child := range t2.After {
		if IsSubtree(t1, child) {
			return true
		}
	}

	return false
}

func CheckCycle(tree OrderTree) error {
	for _, subtree := range tree.After {
		if subtree.Cycle {
			return ErrRunCycleDetected
		}

		err := CheckCycle(subtree)
		if err != nil {
			return err
		}
	}

	return nil
}

func buildOrderTree(stack Stack, visited map[string]struct{}) (OrderTree, error) {
	root := OrderTree{
		Stack: stack,
	}

	if _, ok := visited[stack.Dir]; ok {
		root.Cycle = true
		return root, nil
	}
	visited[stack.Dir] = struct{}{}

	afterStacks, err := LoadStacks(stack.Dir, stack.After...)
	if err != nil {
		return OrderTree{}, err
	}

	for _, s := range afterStacks {
		if _, ok := visited[s.Dir]; ok {
			// cycle detected, dont recurse anymore
			root.After = append(root.After, OrderTree{
				Stack: s,
				Cycle: true,
			})
			continue
		}

		tree, err := buildOrderTree(s, copyVisited(visited))
		if err != nil {
			return OrderTree{}, fmt.Errorf("computing tree of stack %q: %w",
				stack.Dir, err)
		}

		root.After = append(root.After, tree)
	}

	return root, nil
}

func copyVisited(v map[string]struct{}) map[string]struct{} {
	v2 := map[string]struct{}{}
	for k := range v {
		v2[k] = struct{}{}
	}
	return v2
}
