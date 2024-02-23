// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package run

import (
	"cmp"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/run/dag"
	"golang.org/x/exp/slices"
)

// Sort computes the final execution order for the given list of stacks.
// In the case of multiple possible orders, it returns the lexicographic sorted path.
func Sort[S ~[]E, E any](root *config.Root, items S, getStack func(E) *config.Stack) (string, error) {
	d, reason, err := buildValidStackDAG(root, items, getStack)
	if err != nil {
		return reason, err
	}

	getStackDir := func(s E) string {
		return getStack(s).Dir.String()
	}

	order := d.Order()
	orderLookup := make(map[string]int, len(order))
	for idx, id := range order {
		s, err := d.Node(id)
		if err != nil {
			return "", fmt.Errorf("calculating run-order: %w", err)
		}
		orderLookup[s.Dir.String()] = idx
	}

	slices.SortStableFunc(items, func(a, b E) int {
		return cmp.Compare(orderLookup[getStackDir(a)], orderLookup[getStackDir(b)])
	})

	return "", nil
}

// BuildDAGFromStacks computes the final, reduced dag for the given list of stacks.
func BuildDAGFromStacks[S ~[]E, E any](root *config.Root, items S, getStack func(E) *config.Stack) (*dag.DAG[E], string, error) {
	d, reason, err := buildValidStackDAG(root, items, getStack)
	if err != nil {
		return nil, reason, err
	}

	getStackDir := func(s E) string {
		return getStack(s).Dir.String()
	}

	// Minimize graph by removing stacks that were only pulled in for ordering,
	// but are not executed.
	d.Reduce(func(id dag.ID) bool {
		s, err := d.Node(id)
		if err != nil {
			return false
		}
		return !slices.ContainsFunc(items, func(item E) bool {
			return getStackDir(item) == s.Dir.String()
		})
	})

	itemLookup := make(map[string]E, len(items))
	for _, item := range items {
		itemLookup[getStackDir(item)] = item
	}

	// Transform from DAG of stacks to their corresponding E value.
	// We have to build the DAG with stacks first, because for the nodes that were pulled in
	// as depdencies, we have no E value (i.e. these are not in items).
	// After the graph has been reduced, we can look up the corresponding E values.
	newD, err := dag.Transform[E](d, func(id dag.ID, s *config.Stack) (E, error) {
		e, found := itemLookup[s.Dir.String()]
		if !found {
			return e, fmt.Errorf("failed to transform run-order graph")
		}
		return e, nil
	})
	if err != nil {
		return nil, "", err
	}

	return newD, "", nil
}

func buildValidStackDAG[S ~[]E, E any](
	root *config.Root,
	items S,
	getStack func(E) *config.Stack,
) (*dag.DAG[*config.Stack], string, error) {
	d := dag.New[*config.Stack]()

	logger := log.With().
		Str("action", "run.buildOrderedStackDAG()").
		Str("root", root.HostDir()).
		Logger()

	isParentStack := func(s1, s2 *config.Stack) bool {
		return s1.Dir.HasPrefix(s2.Dir.String() + "/")
	}

	getStackDir := func(s E) string {
		return getStack(s).Dir.String()
	}

	slices.SortStableFunc(items, func(a, b E) int {
		return strings.Compare(getStack(a).Dir.String(), getStack(b).Dir.String())
	})

	for _, a := range items {
		for _, b := range items {
			if getStack(a).Dir == getStack(b).Dir {
				continue
			}

			if isParentStack(getStack(a), getStack(b)) {
				logger.Debug().Msgf("stack %q runs before %q since it is its parent", getStackDir(a), getStackDir(b))

				getStack(b).AppendBefore(getStack(a).Dir.String())
			}
		}
	}

	visited := dag.Visited{}
	for _, elem := range items {
		if _, ok := visited[dag.ID(getStack(elem).Dir.String())]; ok {
			continue
		}

		logger.Debug().
			Stringer("stack", getStack(elem).Dir).
			Msg("Build DAG.")

		err := BuildDAG(
			d,
			root,
			getStack(elem),
			"before",
			func(s config.Stack) []string { return s.Before },
			"after",
			func(s config.Stack) []string { return s.After },
			visited,
		)

		if err != nil {
			return nil, "", err
		}
	}

	reason, err := d.Validate()
	if err != nil {
		return nil, reason, err
	}

	return d, "", nil
}

// BuildDAG builds a run order DAG for the given stack.
func BuildDAG(
	d *dag.DAG[*config.Stack],
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
		Stringer("stack", s.Dir).
		Logger()

	if _, ok := visited[dag.ID(s.Dir.String())]; ok {
		return nil
	}

	visited[dag.ID(s.Dir.String())] = struct{}{}

	computePaths := func(fieldname string, paths []string) ([]string, error) {
		uniqPaths := map[string]struct{}{}
		for _, pathstr := range paths {
			if strings.HasPrefix(pathstr, "tag:") {
				if fieldname != "before" && fieldname != "after" {
					return nil, errors.E(
						"tag:<query> is not allowed in %q field", fieldname,
					)
				}
				filter := strings.TrimPrefix(pathstr, "tag:")
				stacksPaths, err := root.StacksByTagsFilters([]string{filter})
				if err != nil {
					return nil, errors.E(err, "invalid order entry %q", pathstr)
				}
				for _, stackPath := range stacksPaths {
					uniqPaths[stackPath.String()] = struct{}{}
				}
				continue
			}

			var abspath string
			if path.IsAbs(pathstr) {
				abspath = filepath.Join(root.HostDir(), filepath.FromSlash(pathstr))
			} else {
				abspath = filepath.Join(s.HostDir(root), filepath.FromSlash(pathstr))
			}
			st, err := os.Stat(abspath)
			if err != nil {
				printer.Stderr.WarnWithDetails(
					fmt.Sprintf("Stack references invalid path in '%s' attribute", fieldname),
					err,
				)
			} else if !st.IsDir() {
				printer.Stderr.WarnWithDetails(
					fmt.Sprintf("Stack references invalid path in '%s' attribute", fieldname),
					errors.E("Path %s is not a directory", pathstr),
				)
			} else {
				uniqPaths[pathstr] = struct{}{}
			}
		}

		var cleanpaths []string
		for path := range uniqPaths {
			cleanpaths = append(cleanpaths, path)
		}
		return cleanpaths, nil
	}

	errs := errors.L()
	ancestorPaths, err := computePaths(ancestorsName, getAncestors(*s))
	errs.Append(err)
	descendantPaths, err := computePaths(descendantsName, getDescendants(*s))
	errs.Append(err)

	if err := errs.AsError(); err != nil {
		return err
	}

	ancestorStacks, err := config.StacksFromTrees(root.StacksByPaths(s.Dir, ancestorPaths...))
	if err != nil {
		return errors.E(err, "stack %q: failed to load the \"%s\" stacks",
			s, ancestorsName)
	}
	descendantStacks, err := config.StacksFromTrees(root.StacksByPaths(s.Dir, descendantPaths...))
	if err != nil {
		return errors.E(err, "stack %q: failed to load the \"%s\" stacks",
			s, descendantsName)
	}

	logger.Debug().Msg("Add new node to DAG.")

	err = d.AddNode(dag.ID(s.Dir.String()), s, toids(descendantStacks), toids(ancestorStacks))
	if err != nil {
		return errors.E("stack %q: failed to build DAG: %w", s, err)
	}

	stacks := config.List[*config.SortableStack]{}
	stacks = append(stacks, ancestorStacks...)
	stacks = append(stacks, descendantStacks...)

	for _, elem := range stacks {
		logger = log.With().
			Str("action", "run.BuildDAG()").
			Str("path", root.HostDir()).
			Stringer("stack", elem.Dir()).
			Logger()

		if _, ok := visited[dag.ID(elem.Dir().String())]; ok {
			continue
		}

		err = BuildDAG(d, root, elem.Stack, descendantsName, getDescendants,
			ancestorsName, getAncestors, visited)
		if err != nil {
			return errors.E(err, "stack %q: failed to build DAG", elem)
		}
	}
	return nil
}

func toids(values config.List[*config.SortableStack]) []dag.ID {
	ids := make([]dag.ID, 0, len(values))
	for _, v := range values {
		ids = append(ids, dag.ID(v.Dir().String()))
	}
	return ids
}
