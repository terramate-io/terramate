// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package rungraph provides the run-graph command.
package rungraph

import (
	"context"
	"io"
	"os"

	"github.com/emicklei/dot"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/commands"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/run"
	"github.com/terramate-io/terramate/run/dag"
	"github.com/terramate-io/terramate/stack"
)

// Spec is the command specification for the run-graph command.
type Spec struct {
	Label      string
	OutputFile string

	workingDir string
	engine     *engine.Engine
	printers   printer.Printers
}

// Name returns the name of the command.
func (s *Spec) Name() string { return "experimental run-graph" }

// Requirements returns the requirements of the command.
func (s *Spec) Requirements(context.Context, commands.CLI) any { return commands.RequireEngine() }

// Exec executes the run-graph command.
func (s *Spec) Exec(_ context.Context, cli commands.CLI) error {
	s.workingDir = cli.WorkingDir()
	s.engine = cli.Engine()
	s.printers = cli.Printers()

	var getLabel func(s *config.Stack) string

	logger := log.With().
		Str("action", "generateGraph()").
		Str("workingDir", s.workingDir).
		Logger()

	switch s.Label {
	case "stack.name":
		logger.Debug().Msg("Set label to stack name.")

		getLabel = func(s *config.Stack) string { return s.Name }
	case "stack.dir":
		logger.Debug().Msg("Set label stack directory.")

		getLabel = func(s *config.Stack) string { return s.Dir.String() }
	default:
		return errors.E(`-label expects the values "stack.name" or "stack.dir"`)
	}

	cfg := s.engine.Config()

	entries, err := stack.List(cfg, cfg.Tree())
	if err != nil {
		return errors.E(err, "listing stacks to build graph")
	}

	logger.Debug().Msg("Create new graph.")

	dotGraph := dot.NewGraph(dot.Directed)
	graph := dag.New[*config.Stack]()

	visited := dag.Visited{}

	filteredStacks, err := s.engine.FilterStacks(entries, engine.ByWorkingDir())
	if err != nil {
		return err
	}

	for _, e := range filteredStacks {
		if _, ok := visited[dag.ID(e.Stack.Dir.String())]; ok {
			continue
		}

		if err := run.BuildDAG(
			graph,
			cfg,
			e.Stack,
			"before",
			func(s config.Stack) []string { return s.Before },
			"after",
			func(s config.Stack) []string { return s.After },
			visited,
		); err != nil {
			return errors.E(err, "building order tree")
		}
	}

	for _, id := range graph.IDs() {
		val, err := graph.Node(id)
		if err != nil {
			return errors.E(err, "generating graph")
		}

		err = generateDot(dotGraph, graph, id, val, getLabel)
		if err != nil {
			return err
		}
	}

	logger.Debug().
		Msg("Set output of graph.")
	outFile := s.OutputFile
	var out io.Writer
	if outFile == "" {
		out = s.printers.Stdout
	} else {
		f, err := os.Create(outFile)
		if err != nil {
			return errors.E(err, "opening file %s", outFile)
		}

		defer func() {
			_ = f.Close()
		}()

		out = f
	}

	logger.Debug().Msg("Write graph to output.")

	_, err = out.Write([]byte(dotGraph.String()))
	if err != nil {
		return errors.E(err, "writing output %s", outFile)
	}
	return nil
}

func generateDot(
	dotGraph *dot.Graph,
	graph *dag.DAG[*config.Stack],
	id dag.ID,
	stackval *config.Stack,
	getLabel func(s *config.Stack) string,
) error {
	descendant := dotGraph.Node(getLabel(stackval))
	for _, ancestor := range graph.AncestorsOf(id) {
		s, err := graph.Node(ancestor)
		if err != nil {
			return errors.E(err, "generating dot file")
		}
		ancestorNode := dotGraph.Node(getLabel(s))

		// we invert the graph here.

		edges := dotGraph.FindEdges(ancestorNode, descendant)
		if len(edges) == 0 {
			edge := dotGraph.Edge(ancestorNode, descendant)
			if graph.HasCycle(ancestor) {
				edge.Attr("color", "red")
				continue
			}
		}

		if graph.HasCycle(ancestor) {
			continue
		}

		err = generateDot(dotGraph, graph, ancestor, s, getLabel)
		if err != nil {
			return err
		}
	}
	return nil
}
