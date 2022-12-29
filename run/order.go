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
	"os"
	"path"
	"path/filepath"
	"sort"

	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/run/dag"
	"github.com/rs/zerolog/log"
)

// Sort computes the final execution order for the given list of stacks.
// In the case of multiple possible orders, it returns the lexicographic sorted
// path.
func Sort(root *config.Root, stacks config.List[*config.Stack]) (config.List[*config.Stack], string, error) {
	d := dag.New()

	logger := log.With().
		Str("action", "run.Sort()").
		Str("root", root.HostDir()).
		Logger()

	logger.Trace().Msg("Computes implicit hierarchical order.")

	isParentStack := func(s1, s2 *config.Stack) bool {
		return s1.Dir().HasPrefix(s2.Dir().String() + "/")
	}

	sort.Sort(stacks)
	for _, stack := range stacks {
		for _, other := range stacks {
			if stack.Dir() == other.Dir() {
				continue
			}

			if isParentStack(stack, other) {
				logger.Debug().Msgf("stack %q runs before %q since it is its parent", other, stack)

				other.AppendBefore(stack.Dir().String())
			}
		}
	}

	logger.Trace().Msg("Sorting stacks.")

	visited := dag.Visited{}
	for _, s := range stacks {
		if _, ok := visited[dag.ID(s.Dir().String())]; ok {
			continue
		}

		logger.Debug().
			Stringer("stack", s.Dir()).
			Msg("Build DAG.")

		err := BuildDAG(
			d,
			root,
			s,
			"before",
			config.Stack.Before,
			"after",
			config.Stack.After,
			visited,
		)

		if err != nil {
			return nil, "", err
		}
	}

	logger.Trace().Msg("Validate DAG.")

	reason, err := d.Validate()
	if err != nil {
		return nil, reason, err
	}

	logger.Trace().Msg("Get topologically order DAG.")

	order := d.Order()

	orderedStacks := make(config.List[*config.Stack], 0, len(order))

	logger.Trace().Msg("Get ordered stacks.")

	isSelectedStack := func(s *config.Stack) bool {
		// Stacks may be added on the DAG from after/before references
		// but they should not be on the final order if they are not part
		// of the previously selected stacks passed as a parameter.
		// This is important for change detection to work on ordering and
		// also for filtering by working dir.
		for _, stack := range stacks {
			if s.Dir() == stack.Dir() {
				return true
			}
		}
		return false
	}

	for _, id := range order {
		val, err := d.Node(id)
		if err != nil {
			return nil, "", fmt.Errorf("calculating run-order: %w", err)
		}
		s := val.(*config.Stack)
		if !isSelectedStack(s) {
			logger.Trace().
				Stringer("stack", s.Dir()).
				Msg("ignoring since not part of selected stacks")
			continue
		}
		orderedStacks = append(orderedStacks, s)
	}

	return orderedStacks, "", nil
}

// BuildDAG builds a run order DAG for the given stack.
func BuildDAG(
	d *dag.DAG,
	root *config.Root,
	s *config.Stack,
	descendantsName string,
	getDescendants func(config.Stack) []string,
	ancestorsName string,
	getAncestors func(config.Stack) []string,
	visited dag.Visited,
) error {
	logger := log.With().
		Str("action", "run.BuildDAG()").
		Str("path", root.HostDir()).
		Stringer("stack", s.Dir()).
		Logger()

	if _, ok := visited[dag.ID(s.Dir().String())]; ok {
		return nil
	}

	visited[dag.ID(s.Dir().String())] = struct{}{}

	removeWrongPaths := func(fieldname string, paths []string) []string {
		cleanpaths := []string{}
		for _, pathstr := range paths {
			var abspath string
			if path.IsAbs(pathstr) {
				abspath = filepath.Join(root.HostDir(), filepath.FromSlash(pathstr))
			} else {
				abspath = filepath.Join(s.HostDir(), filepath.FromSlash(pathstr))
			}
			st, err := os.Stat(abspath)
			if err != nil {
				log.Warn().
					Err(err).
					Msgf("building dag: failed to stat %s path %s - ignoring", fieldname, abspath)
			} else if !st.IsDir() {
				log.Warn().
					Msgf("building dag: stack.%s path %s is not a directory - ignoring",
						fieldname, pathstr)
			} else {
				cleanpaths = append(cleanpaths, pathstr)
			}
		}
		return cleanpaths
	}

	ancestorPaths := removeWrongPaths(ancestorsName, getAncestors(*s))
	descendantPaths := removeWrongPaths(descendantsName, getDescendants(*s))

	ancestorStacks, err := config.StacksFromTrees(root.HostDir(), root.StacksByPaths(s.Dir(), ancestorPaths...))
	if err != nil {
		return errors.E(err, "stack %q: failed to load the \"%s\" stacks",
			s, ancestorsName)
	}
	descendantStacks, err := config.StacksFromTrees(root.HostDir(), root.StacksByPaths(s.Dir(), descendantPaths...))
	if err != nil {
		return errors.E(err, "stack %q: failed to load the \"%s\" stacks",
			s, descendantsName)
	}

	logger.Debug().Msg("Add new node to DAG.")

	err = d.AddNode(dag.ID(s.Dir().String()), s, toids(descendantStacks), toids(ancestorStacks))
	if err != nil {
		return errors.E("stack %q: failed to build DAG: %w", s, err)
	}

	stacks := config.List[*config.Stack]{}
	stacks = append(stacks, ancestorStacks...)
	stacks = append(stacks, descendantStacks...)

	logger.Trace().Msg("Range over stacks.")

	for _, s := range stacks {
		logger = log.With().
			Str("action", "run.BuildDAG()").
			Str("path", root.HostDir()).
			Stringer("stack", s.Dir()).
			Logger()

		if _, ok := visited[dag.ID(s.Dir().String())]; ok {
			continue
		}

		logger.Trace().Msg("Build DAG.")

		err = BuildDAG(d, root, s, descendantsName, getDescendants,
			ancestorsName, getAncestors, visited)
		if err != nil {
			return errors.E(err, "stack %q: failed to build DAG", s)
		}
	}
	return nil
}

func toids(values config.List[*config.Stack]) []dag.ID {
	ids := make([]dag.ID, 0, len(values))
	for _, v := range values {
		ids = append(ids, dag.ID(v.Dir().String()))
	}
	return ids
}
