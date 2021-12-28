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
	"github.com/mineiros-io/terramate/dag"
	"github.com/mineiros-io/terramate/stack"
)

// RunOrder computes the final execution order for the given list of stacks.
// In the case of multiple possible orders, it returns the lexicographic sorted
// path.
func RunOrder(root string, stacks []stack.S, changed bool) ([]stack.S, string, error) {
	d := dag.New()
	loader := stack.NewLoader(root)
	for _, stack := range stacks {
		loader.Set(stack.Dir, stack)
	}

	visited := map[string]struct{}{}
	for _, stack := range stacks {
		if _, ok := visited[stack.Dir]; ok {
			continue
		}
		err := BuildDAG(d, root, stack, loader, visited)
		if err != nil {
			return nil, "", err
		}
	}

	reason, err := d.Validate()
	if err != nil {
		return nil, reason, err
	}

	order := d.Order()

	orderedStacks := make([]stack.S, 0, len(order))
	for _, id := range order {
		orderedStacks = append(orderedStacks, loader.MustGet(string(id)))
	}

	return orderedStacks, "", nil
}

func BuildDAG(
	d *dag.DAG,
	rootdir string,
	stackval stack.S,
	loader stack.Loader,
	visited map[string]struct{},
) error {
	visited[stackval.Dir] = struct{}{}

	afterStacks, err := loader.LoadAll(rootdir, stackval.Dir, stackval.After()...)
	if err != nil {
		return err
	}

	beforeStacks, err := loader.LoadAll(rootdir, stackval.Dir, stackval.Before()...)
	if err != nil {
		return err
	}

	err = d.AddVertice(dag.ID(stackval.Dir), stackval, toids(beforeStacks),
		toids(afterStacks))

	if err != nil {
		return err
	}

	stacks := []stack.S{}
	stacks = append(stacks, afterStacks...)
	stacks = append(stacks, beforeStacks...)

	for _, s := range stacks {
		if _, ok := visited[s.Dir]; ok {
			continue
		}

		err = BuildDAG(d, rootdir, s, loader, visited)
		if err != nil {
			return err
		}
	}
	return nil
}

func toids(values []stack.S) []dag.ID {
	ids := make([]dag.ID, 0, len(values))
	for _, v := range values {
		ids = append(ids, dag.ID(v.Dir))
	}
	return ids
}
