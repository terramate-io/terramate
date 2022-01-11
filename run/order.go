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
	"fmt"

	"github.com/mineiros-io/terramate/run/dag"
	"github.com/mineiros-io/terramate/stack"
	"github.com/rs/zerolog/log"
)

type visited map[string]struct{}

// Sort computes the final execution order for the given list of stacks.
// In the case of multiple possible orders, it returns the lexicographic sorted
// path.
func Sort(root string, stacks []stack.S, changed bool) ([]stack.S, string, error) {
	logger := log.With().
		Str("action", "RunOrder()").
		Str("path", root).
		Logger()

	logger.Debug().
		Msg("Create new directed acyclic graph.")
	d := dag.New()

	logger.Trace().
		Msg("Create new stack loader.")
	loader := stack.NewLoader(root)

	logger.Trace().
		Msg("Add stacks to loader.")
	for _, stack := range stacks {
		loader.Set(stack.Dir, stack)
	}

	visited := visited{}

	logger.Trace().
		Msg("Range over stacks.")
	for _, stack := range stacks {
		if _, ok := visited[stack.Dir]; ok {
			continue
		}

		logger.Debug().
			Str("stack", stack.Dir).
			Msg("Build DAG.")
		err := BuildDAG(d, root, stack, loader, visited)
		if err != nil {
			return nil, "", err
		}
	}

	logger.Trace().
		Msg("Validate DAG.")
	reason, err := d.Validate()
	if err != nil {
		return nil, reason, err
	}

	logger.Trace().
		Msg("Get topologically order DAG.")
	order := d.Order()

	orderedStacks := make([]stack.S, 0, len(order))

	logger.Trace().
		Msg("Get ordered stacks.")
	for _, id := range order {
		val, err := d.Node(id)
		if err != nil {
			return nil, "", fmt.Errorf("calculating run-order: %w", err)
		}
		s := val.(stack.S)
		if s.IsChanged() == changed {
			orderedStacks = append(orderedStacks, s)
		}
	}

	return orderedStacks, "", nil
}

func BuildDAG(
	d *dag.DAG,
	root string,
	s stack.S,
	loader stack.Loader,
	visited visited,
) error {
	logger := log.With().
		Str("action", "BuildDAG()").
		Str("path", root).
		Str("stack", s.Dir).
		Logger()

	visited[s.Dir] = struct{}{}

	logger.Trace().
		Msg("Load all stacks in dir after current stack.")
	afterStacks, err := loader.LoadAll(root, s.Dir, s.After()...)
	if err != nil {
		return err
	}

	logger.Trace().
		Msg("Load all stacks in dir before current stack.")
	beforeStacks, err := loader.LoadAll(root, s.Dir, s.Before()...)
	if err != nil {
		return err
	}

	logger.Debug().
		Msg("Add new node to DAG.")
	err = d.AddNode(dag.ID(s.Dir), s, toids(beforeStacks), toids(afterStacks))
	if err != nil {
		return err
	}

	stacks := []stack.S{}
	stacks = append(stacks, afterStacks...)
	stacks = append(stacks, beforeStacks...)

	logger.Trace().
		Msg("Range over stacks.")
	for _, s := range stacks {
		logger = log.With().
			Str("action", "BuildDAG()").
			Str("path", root).
			Str("stack", s.Dir).
			Logger()

		if _, ok := visited[s.Dir]; ok {
			continue
		}

		logger.Trace().
			Msg("Build DAG.")
		err = BuildDAG(d, root, s, loader, visited)
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
