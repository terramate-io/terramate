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
	"sort"

	"github.com/mineiros-io/terramate/stack"
)

// OrderDAG represents the Directed Acyclic Graph of the stack order.
type OrderDAG struct {
	Stack stack.S    // Stack is the stack which is the root of this DAG.
	Order []OrderDAG // After is the list of depend-on DAG trees.

	Cycle bool // Cycle tells if a cycle was detected at this level.
}

// BuildOrderTree builds the order tree data structure.
func BuildOrderTree(stack stack.S, l stack.Loader) (OrderDAG, error) {
	return buildOrderTree(stack, l, map[string]struct{}{})
}

// RunOrder computes the final execution order for the given list of stacks.
// In the case of multiple possible orders, it returns the lexicographic sorted
// path.
func RunOrder(stacks []stack.S, changed bool) ([]stack.S, error) {
	trees := map[string]OrderDAG{} // indexed by stackdir

	loader := stack.NewLoader()
	for _, stack := range stacks {
		loader.Set(stack.Dir, stack)
	}

	for _, stack := range stacks {
		tree, err := BuildOrderTree(stack, loader)
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

	order := []stack.S{}
	visited := map[string]struct{}{}
	for _, k := range keys {
		tree := trees[k]
		walkOrderTree(tree, func(s stack.S) {
			if _, ok := visited[s.Dir]; !ok {
				visited[s.Dir] = struct{}{}

				if changed == s.IsChanged() {
					order = append(order, s)
				}
			}
		})
	}

	return order, nil
}

func walkOrderTree(tree OrderDAG, do func(s stack.S)) {
	for _, child := range tree.Order {
		walkOrderTree(child, do)
	}

	do(tree.Stack)
}

func IsSubtree(t1, t2 OrderDAG) bool {
	if t1.Stack.Dir == t2.Stack.Dir {
		return true
	}
	for _, child := range t2.Order {
		if IsSubtree(t1, child) {
			return true
		}
	}

	return false
}

// CheckCycle tells if the graph has cycles.
func CheckCycle(tree OrderDAG) error {
	for _, subtree := range tree.Order {
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

func buildOrderTree(
	stack stack.S,
	loader stack.Loader,
	visited map[string]struct{},
) (OrderDAG, error) {
	root := OrderDAG{
		Stack: stack,
	}

	visited[stack.Dir] = struct{}{}
	afterStacks, err := loader.LoadAll(stack.Dir, stack.After()...)
	if err != nil {
		return OrderDAG{}, err
	}

	for _, s := range afterStacks {
		if _, ok := visited[s.Dir]; ok {
			// cycle detected, dont recurse anymore
			root.Order = append(root.Order, OrderDAG{
				Stack: s,
				Cycle: true,
			})
			continue
		}

		tree, err := buildOrderTree(s, loader, copyVisited(visited))
		if err != nil {
			return OrderDAG{}, fmt.Errorf("computing tree of stack %q: %w",
				stack.Dir, err)
		}

		root.Order = append(root.Order, tree)
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
