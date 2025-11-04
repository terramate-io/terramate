// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package engine

import (
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/tg"
)

// Modules is an alias for tg.Modules to avoid import cycles
type Modules = tg.Modules

// LoadTerragruntModules is a helper to load Terragrunt modules
func LoadTerragruntModules(rootdir string, dir project.Path) (Modules, error) {
	return tg.ScanModules(rootdir, dir, true)
}

// DependencyGraph represents the dependency relationships between stacks.
type DependencyGraph struct {
	// dependencies maps a stack path to its direct dependencies (what it depends on)
	dependencies map[string][]string

	// dependents maps a stack path to its direct dependents (what depends on it)
	dependents map[string][]string
}

// NewDependencyGraph creates a new dependency graph from the given stacks.
// It extracts ONLY data dependencies from:
// - input.from_stack_id (Terramate native output sharing)
// - Terragrunt dependency blocks (read directly from mod.DependencyBlocks)
//
// NOTE: stack.After and stack.Before are NOT included because they contain ordering-only
// dependencies (e.g., Terragrunt dependencies.paths) which should NOT widen scope.
func (e *Engine) NewDependencyGraph(stacks config.List[*config.SortableStack], tgModules tg.Modules, target string) (*DependencyGraph, error) {
	logger := log.With().
		Str("action", "engine.NewDependencyGraph()").
		Logger()

	graph := &DependencyGraph{
		dependencies: make(map[string][]string),
		dependents:   make(map[string][]string),
	}

	rootcfg := e.Config()

	// Build a map of stack paths for quick lookup
	stackPaths := make(map[string]bool)
	for _, st := range stacks {
		stackPaths[st.Stack.Dir.String()] = true
		// Initialize empty slices to ensure all stacks are in the map
		graph.dependencies[st.Stack.Dir.String()] = []string{}
		graph.dependents[st.Stack.Dir.String()] = []string{}
	}

	// NOTE: We do NOT extract dependencies from stack.After and stack.Before here
	// because those fields include ordering-only dependencies (e.g., Terragrunt dependencies.paths)
	// which should NOT widen scope for --include-all-dependencies flags.
	//
	// Data dependencies are extracted from:
	// - input.from_stack_id (Terramate native output sharing)
	// - Terragrunt dependency blocks (mod.DependencyBlocks - data dependencies only)

	// Extract dependencies from input.from_stack_id (output sharing)
	for _, st := range stacks {
		stackPath := st.Stack.Dir.String()

		// Add dependencies from input.from_stack_id (output sharing)
		cfg, _ := rootcfg.Lookup(st.Stack.Dir)
		for _, inputcfg := range cfg.Node.Inputs {
			evalctx, err := e.SetupEvalContext(e.wd(), st.Stack, target, map[string]string{})
			if err != nil {
				return nil, errors.E(err, "setting up evaluation context for stack %s", stackPath)
			}
			fromStackID, err := config.EvalInputFromStackID(evalctx, inputcfg)
			if err != nil {
				return nil, errors.E(err, "evaluating input.%s.from_stack_id for stack %s", inputcfg.Name, stackPath)
			}

			// Find the stack with this ID
			mgr := e.stackManager()
			depStack, found, err := mgr.StackByID(fromStackID)
			if err != nil {
				return nil, errors.E(err, "looking up stack by ID %s", fromStackID)
			}
			if !found {
				logger.Warn().
					Str("stack", stackPath).
					Str("from_stack_id", fromStackID).
					Msg("stack referenced in input.from_stack_id not found")
				continue
			}

			depPath := depStack.Dir.String()
			if !stackPaths[depPath] {
				logger.Debug().
					Str("stack", stackPath).
					Str("dependency", depPath).
					Msg("dependency from input.from_stack_id not in current stack set, skipping")
				continue
			}

			graph.addDependency(stackPath, depPath)
		}
	}

	// Extract dependencies from Terragrunt dependency blocks (data dependencies only)
	// These are read directly from mod.DependencyBlocks without requiring input blocks
	for _, mod := range tgModules {
		// Find the stack corresponding to this module
		modStackPath := mod.Path.String()
		if _, isStack := stackPaths[modStackPath]; !isStack {
			// Module doesn't correspond to a stack in the current set, skip
			continue
		}

		// Process each dependency block path
		for _, depPath := range mod.DependencyBlocks {
			depStackPath := depPath.String()
			if !stackPaths[depStackPath] {
				logger.Debug().
					Str("stack", modStackPath).
					Str("dependency", depStackPath).
					Msg("dependency from Terragrunt dependency block not in current stack set, skipping")
				continue
			}

			graph.addDependency(modStackPath, depStackPath)
		}
	}

	logger.Debug().
		Int("stacks", len(stacks)).
		Int("dependency_edges", graph.countEdges()).
		Msg("dependency graph constructed")

	return graph, nil
}

// addDependency adds a dependency edge and its inverse dependent edge
func (g *DependencyGraph) addDependency(stack, dependency string) {
	// Check if already exists to avoid duplicates
	for _, existing := range g.dependencies[stack] {
		if existing == dependency {
			return
		}
	}

	g.dependencies[stack] = append(g.dependencies[stack], dependency)
	g.dependents[dependency] = append(g.dependents[dependency], stack)
}

// countEdges returns the total number of dependency edges
func (g *DependencyGraph) countEdges() int {
	count := 0
	for _, deps := range g.dependencies {
		count += len(deps)
	}
	return count
}

// GetDirectDependencies returns the immediate dependencies of a stack (what it depends on)
func (g *DependencyGraph) GetDirectDependencies(stackPath string) []string {
	deps := g.dependencies[stackPath]
	if deps == nil {
		return []string{}
	}
	// Return a copy to prevent modification
	result := make([]string, len(deps))
	copy(result, deps)
	return result
}

// GetDirectDependents returns the immediate dependents of a stack (what depends on it)
func (g *DependencyGraph) GetDirectDependents(stackPath string) []string {
	deps := g.dependents[stackPath]
	if deps == nil {
		return []string{}
	}
	// Return a copy to prevent modification
	result := make([]string, len(deps))
	copy(result, deps)
	return result
}

// GetAllDependencies returns all transitive dependencies of a stack (what it depends on, recursively)
func (g *DependencyGraph) GetAllDependencies(stackPath string) []string {
	visited := make(map[string]bool)
	result := []string{}

	var traverse func(string)
	traverse = func(path string) {
		if visited[path] {
			return
		}
		visited[path] = true

		for _, dep := range g.dependencies[path] {
			if !visited[dep] {
				result = append(result, dep)
				traverse(dep)
			}
		}
	}

	traverse(stackPath)
	return result
}

// GetAllDependents returns all transitive dependents of a stack (what depends on it, recursively)
func (g *DependencyGraph) GetAllDependents(stackPath string) []string {
	visited := make(map[string]bool)
	result := []string{}

	var traverse func(string)
	traverse = func(path string) {
		if visited[path] {
			return
		}
		visited[path] = true

		for _, dep := range g.dependents[path] {
			if !visited[dep] {
				result = append(result, dep)
				traverse(dep)
			}
		}
	}

	traverse(stackPath)
	return result
}

// GetDirectDependenciesForStacks returns all direct dependencies for a set of stacks
func (g *DependencyGraph) GetDirectDependenciesForStacks(stackPaths []string) []string {
	depMap := make(map[string]bool)

	for _, stackPath := range stackPaths {
		for _, dep := range g.GetDirectDependencies(stackPath) {
			depMap[dep] = true
		}
	}

	result := make([]string, 0, len(depMap))
	for dep := range depMap {
		result = append(result, dep)
	}
	return result
}

// GetAllDependenciesForStacks returns all transitive dependencies for a set of stacks
func (g *DependencyGraph) GetAllDependenciesForStacks(stackPaths []string) []string {
	depMap := make(map[string]bool)

	for _, stackPath := range stackPaths {
		for _, dep := range g.GetAllDependencies(stackPath) {
			depMap[dep] = true
		}
	}

	result := make([]string, 0, len(depMap))
	for dep := range depMap {
		result = append(result, dep)
	}
	return result
}

// GetDirectDependentsForStacks returns all direct dependents for a set of stacks
func (g *DependencyGraph) GetDirectDependentsForStacks(stackPaths []string) []string {
	depMap := make(map[string]bool)

	for _, stackPath := range stackPaths {
		for _, dep := range g.GetDirectDependents(stackPath) {
			depMap[dep] = true
		}
	}

	result := make([]string, 0, len(depMap))
	for dep := range depMap {
		result = append(result, dep)
	}
	return result
}

// GetAllDependentsForStacks returns all transitive dependents for a set of stacks
func (g *DependencyGraph) GetAllDependentsForStacks(stackPaths []string) []string {
	depMap := make(map[string]bool)

	for _, stackPath := range stackPaths {
		for _, dep := range g.GetAllDependents(stackPath) {
			depMap[dep] = true
		}
	}

	result := make([]string, 0, len(depMap))
	for dep := range depMap {
		result = append(result, dep)
	}
	return result
}

// DetectCycles detects circular dependencies in the graph
// Returns the paths involved in cycles, if any
func (g *DependencyGraph) DetectCycles() [][]string {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	cycles := [][]string{}
	currentPath := []string{}

	var visit func(string) bool
	visit = func(node string) bool {
		visited[node] = true
		recStack[node] = true
		currentPath = append(currentPath, node)

		for _, dep := range g.dependencies[node] {
			if !visited[dep] {
				if visit(dep) {
					return true
				}
			} else if recStack[dep] {
				// Found a cycle
				cycleStart := -1
				for i, p := range currentPath {
					if p == dep {
						cycleStart = i
						break
					}
				}
				if cycleStart >= 0 {
					cycle := make([]string, len(currentPath)-cycleStart)
					copy(cycle, currentPath[cycleStart:])
					cycles = append(cycles, cycle)
				}
				return true
			}
		}

		currentPath = currentPath[:len(currentPath)-1]
		recStack[node] = false
		return false
	}

	for node := range g.dependencies {
		if !visited[node] {
			visit(node)
		}
	}

	return cycles
}
