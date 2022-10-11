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
	"github.com/mineiros-io/terramate/stack"
	"github.com/rs/zerolog/log"
)

// Sort computes the final execution order for the given list of stacks.
// In the case of multiple possible orders, it returns the lexicographic sorted
// path.
func Sort(cfg *config.Tree, stacks stack.List) (stack.List, string, error) {
	d := dag.New()

	logger := log.With().
		Str("action", "run.Sort()").
		Str("root", cfg.RootDir()).
		Logger()

	logger.Trace().Msg("Computes implicit hierarchical order.")

	isParentStack := func(s1, s2 *stack.S) bool {
		return s1.Path().HasPrefix(s2.Path().String() + "/")
	}

	sort.Sort(stacks)
	for _, stack := range stacks {
		for _, other := range stacks {
			if stack.Path() == other.Path() {
				continue
			}

			if isParentStack(stack, other) {
				logger.Debug().Msgf("stack %q runs before %q since it is its parent", other, stack)

				other.AppendBefore(stack.Path().String())
			}
		}
	}

	logger.Trace().Msg("Sorting stacks.")

	visited := dag.Visited{}
	for _, s := range stacks {
		if _, ok := visited[dag.ID(s.Path())]; ok {
			continue
		}

		logger.Debug().
			Stringer("stack", s.Path()).
			Msg("Build DAG.")

		err := BuildDAG(
			d,
			cfg,
			s,
			"before",
			stack.S.Before,
			"after",
			stack.S.After,
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

	orderedStacks := make(stack.List, 0, len(order))

	logger.Trace().Msg("Get ordered stacks.")

	isSelectedStack := func(s *stack.S) bool {
		// Stacks may be added on the DAG from after/before references
		// but they should not be on the final order if they are not part
		// of the previously selected stacks passed as a parameter.
		// This is important for change detection to work on ordering and
		// also for filtering by working dir.
		for _, stack := range stacks {
			if s.Path() == stack.Path() {
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
		s := val.(*stack.S)
		if !isSelectedStack(s) {
			logger.Trace().
				Stringer("stack", s.Path()).
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
	cfg *config.Tree,
	s *stack.S,
	descendantsName string,
	getDescendants func(stack.S) []string,
	ancestorsName string,
	getAncestors func(stack.S) []string,
	visited dag.Visited,
) error {
	logger := log.With().
		Str("action", "BuildDAG()").
		Str("path", cfg.RootDir()).
		Stringer("stack", s.Path()).
		Logger()

	if _, ok := visited[dag.ID(s.Path())]; ok {
		return nil
	}

	visited[dag.ID(s.Path())] = struct{}{}

	removeWrongPaths := func(fieldname string, paths []string) []string {
		cleanpaths := []string{}
		for _, pathstr := range paths {
			var abspath string
			if path.IsAbs(pathstr) {
				abspath = filepath.Join(cfg.RootDir(), filepath.FromSlash(pathstr))
			} else {
				abspath = filepath.Join(s.HostPath(), filepath.FromSlash(pathstr))
			}
			st, err := os.Stat(abspath)
			if err != nil {
				logger.Warn().
					Err(err).
					Msgf("failed to stat %s path %s - ignoring", fieldname, abspath)
			} else if !st.IsDir() {
				logger.Warn().
					Msgf("stack.%s path %s is not a directory - ignoring",
						fieldname, pathstr)
			} else {
				cleanpaths = append(cleanpaths, pathstr)
			}
		}
		return cleanpaths
	}

	ancestorPaths := removeWrongPaths(ancestorsName, getAncestors(*s))
	descendantPaths := removeWrongPaths(descendantsName, getDescendants(*s))

	ancestorStacks, err := stack.TreeListToStackList(cfg.RootDir(), cfg.StacksByPaths(s.Path(), ancestorPaths...))
	if err != nil {
		return errors.E(err, "stack %q: failed to load the \"%s\" stacks",
			s, ancestorsName)
	}
	descendantStacks, err := stack.TreeListToStackList(cfg.RootDir(), cfg.StacksByPaths(s.Path(), descendantPaths...))
	if err != nil {
		return errors.E(err, "stack %q: failed to load the \"%s\" stacks",
			s, descendantsName)
	}

	logger.Debug().Msg("Add new node to DAG.")

	err = d.AddNode(dag.ID(s.Path()), s, toids(descendantStacks), toids(ancestorStacks))
	if err != nil {
		return errors.E("stack %q: failed to build DAG: %w", s, err)
	}

	stacks := stack.List{}
	stacks = append(stacks, ancestorStacks...)
	stacks = append(stacks, descendantStacks...)

	logger.Trace().Msg("Range over stacks.")

	for _, s := range stacks {
		logger = log.With().
			Str("action", "run.BuildDAG()").
			Str("path", cfg.RootDir()).
			Stringer("stack", s.Path()).
			Logger()

		if _, ok := visited[dag.ID(s.Path())]; ok {
			continue
		}

		logger.Trace().Msg("Build DAG.")

		err = BuildDAG(d, cfg, s, descendantsName, getDescendants,
			ancestorsName, getAncestors, visited)
		if err != nil {
			return errors.E(err, "stack %q: failed to build DAG", s)
		}
	}
	return nil
}

func toids(values stack.List) []dag.ID {
	ids := make([]dag.ID, 0, len(values))
	for _, v := range values {
		ids = append(ids, dag.ID(v.Path()))
	}
	return ids
}
