// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stdlib

import (
	"sort"

	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

// TreeFunc returns the `tm_tree` function spec.
// This function builds a tree/forest from a list of [parent, child] pairs
// and returns all branch paths from root to each node.
func TreeFunc() function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "pairs",
				Type: cty.List(cty.Tuple([]cty.Type{
					cty.String, // parent (can be null)
					cty.String, // child
				})),
			},
		},
		Type: function.StaticReturnType(cty.List(cty.List(cty.String))),
		Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
			return buildTree(args[0])
		},
	})
}

// treeNode represents a node in the tree
type treeNode struct {
	name     string
	parent   *treeNode
	children []*treeNode
}

// buildTree processes the input pairs and builds a tree/forest
func buildTree(pairsVal cty.Value) (cty.Value, error) {
	if pairsVal.IsNull() {
		return cty.ListValEmpty(cty.List(cty.String)), nil
	}

	pairs := pairsVal.AsValueSlice()
	if len(pairs) == 0 {
		return cty.ListValEmpty(cty.List(cty.String)), nil
	}

	// Parse pairs and build node registry
	nodes := make(map[string]*treeNode)
	parentChildMap := make(map[string]string) // child -> parent (use "<null>" for roots)
	childrenMap := make(map[string][]string)  // parent -> []children
	allNodeNames := make(map[string]bool)     // track all nodes mentioned

	for _, pair := range pairs {
		pairElements := pair.AsValueSlice()
		if len(pairElements) != 2 {
			return cty.NilVal, TreeErrorInvalidPairLength()
		}

		parentVal := pairElements[0]
		childVal := pairElements[1]

		if childVal.IsNull() {
			return cty.NilVal, TreeErrorNullChildValue()
		}

		child := childVal.AsString()
		if child == "" {
			return cty.NilVal, TreeErrorEmptyChild()
		}

		// Track all node names
		allNodeNames[child] = true

		// Determine parent value for tracking
		var parentForTracking string
		if parentVal.IsNull() {
			parentForTracking = "<root>"
		} else {
			parent := parentVal.AsString()
			if parent == "" {
				return cty.NilVal, TreeErrorEmptyParent()
			}
			parentForTracking = parent
			allNodeNames[parent] = true
		}

		// Check for conflicting parents
		if existingParent, exists := parentChildMap[child]; exists {
			return cty.NilVal, TreeErrorConflictingParentsValue(child, existingParent, parentForTracking)
		}

		// Record the parent-child relationship
		parentChildMap[child] = parentForTracking

		// Create child node if it doesn't exist
		if _, exists := nodes[child]; !exists {
			nodes[child] = &treeNode{name: child}
		}

		// Handle non-null parent relationships
		if !parentVal.IsNull() {
			parent := parentVal.AsString()
			childrenMap[parent] = append(childrenMap[parent], child)

			// Create parent node if it doesn't exist
			if _, exists := nodes[parent]; !exists {
				nodes[parent] = &treeNode{name: parent}
			}
		}
	}

	// Check for unknown parents - every parent referenced must appear as a child
	// in some pair, OR be explicitly defined as a root (with null parent)
	for child, parent := range parentChildMap {
		// Skip root nodes (those with <root> parent)
		if parent == "<root>" {
			continue
		}

		// Check if this parent appears as a child somewhere
		_, hasParentDefinition := parentChildMap[parent]

		if !hasParentDefinition {
			return cty.NilVal, TreeErrorUnknownParentValue(parent, child)
		}
	}

	// Build parent-child relationships
	for child, parent := range parentChildMap {
		// Skip root nodes (those with <root> parent)
		if parent == "<root>" {
			continue
		}
		childNode := nodes[child]
		parentNode := nodes[parent]
		childNode.parent = parentNode
		parentNode.children = append(parentNode.children, childNode)
	}

	// Sort children for deterministic output
	for _, node := range nodes {
		sort.Slice(node.children, func(i, j int) bool {
			return node.children[i].name < node.children[j].name
		})
	}

	// Detect cycles using DFS
	visited := make(map[string]bool)
	recursionStack := make(map[string]bool)

	var detectCycle func(nodeName string) error
	detectCycle = func(nodeName string) error {
		if recursionStack[nodeName] {
			return TreeErrorCycleNode(nodeName)
		}
		if visited[nodeName] {
			return nil
		}

		visited[nodeName] = true
		recursionStack[nodeName] = true

		for _, child := range childrenMap[nodeName] {
			if err := detectCycle(child); err != nil {
				return err
			}
		}

		recursionStack[nodeName] = false
		return nil
	}

	for nodeName := range nodes {
		if !visited[nodeName] {
			if err := detectCycle(nodeName); err != nil {
				return cty.NilVal, err
			}
		}
	}

	// Find all root nodes (nodes with no parent)
	var roots []*treeNode
	for _, node := range nodes {
		if node.parent == nil {
			roots = append(roots, node)
		}
	}

	// Sort roots for deterministic output
	sort.Slice(roots, func(i, j int) bool {
		return roots[i].name < roots[j].name
	})

	// Generate all paths from roots to leaves (and intermediate nodes)
	var allPaths [][]string

	var generatePaths func(node *treeNode, currentPath []string)
	generatePaths = func(node *treeNode, currentPath []string) {
		newPath := append(currentPath, node.name)

		// Add the current path (every node gets its own path)
		pathCopy := make([]string, len(newPath))
		copy(pathCopy, newPath)
		allPaths = append(allPaths, pathCopy)

		// Recursively generate paths for children
		for _, child := range node.children {
			generatePaths(child, newPath)
		}
	}

	for _, root := range roots {
		generatePaths(root, []string{})
	}

	// Sort all paths lexicographically for deterministic output
	sort.Slice(allPaths, func(i, j int) bool {
		return comparePaths(allPaths[i], allPaths[j]) < 0
	})

	// Convert to cty values
	var pathValues []cty.Value
	for _, path := range allPaths {
		var stringValues []cty.Value
		for _, node := range path {
			stringValues = append(stringValues, cty.StringVal(node))
		}
		pathValues = append(pathValues, cty.ListVal(stringValues))
	}

	return cty.ListVal(pathValues), nil
}

// comparePaths compares two paths lexicographically
func comparePaths(a, b []string) int {
	minLen := min(len(b), len(a))

	for i := range minLen {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}

	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return 0
}
