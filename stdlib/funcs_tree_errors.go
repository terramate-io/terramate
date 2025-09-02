// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stdlib

import (
	"fmt"
	"strings"
)

// TreeError represents errors that can occur in the tm_tree function
type TreeError struct {
	Type TreeErrorType
	Msg  string
}

// TreeErrorType represents the type of tree error
type TreeErrorType int

const (
	// TreeErrorInvalidInput indicates malformed input data
	TreeErrorInvalidInput TreeErrorType = iota
	// TreeErrorConflictingParents indicates a node has multiple parents
	TreeErrorConflictingParents
	// TreeErrorUnknownParent indicates a parent reference that doesn't exist
	TreeErrorUnknownParent
	// TreeErrorCycle indicates a cycle was detected in the tree
	TreeErrorCycle
	// TreeErrorNullChild indicates a null value was provided for a child
	TreeErrorNullChild
	// TreeErrorEmptyString indicates an empty string was provided for parent or child
	TreeErrorEmptyString
)

// Error implements the error interface
func (e *TreeError) Error() string {
	return e.Msg
}

// IsTreeError checks if an error is a TreeError of a specific type
func IsTreeError(err error, errType TreeErrorType) bool {
	if treeErr, ok := err.(*TreeError); ok {
		return treeErr.Type == errType
	}
	return false
}

// NewTreeError creates a new TreeError
func NewTreeError(errType TreeErrorType, format string, args ...interface{}) *TreeError {
	return &TreeError{
		Type: errType,
		Msg:  fmt.Sprintf(format, args...),
	}
}

// TreeErrorInvalidPairLength creates an error for invalid pair length
func TreeErrorInvalidPairLength() *TreeError {
	return NewTreeError(TreeErrorInvalidInput, "each pair must have exactly 2 elements")
}

// TreeErrorNullChildValue creates an error for null child value
func TreeErrorNullChildValue() *TreeError {
	return NewTreeError(TreeErrorNullChild, "child cannot be null")
}

// TreeErrorEmptyChild creates an error for empty child string
func TreeErrorEmptyChild() *TreeError {
	return NewTreeError(TreeErrorEmptyString, "child cannot be empty string")
}

// TreeErrorEmptyParent creates an error for empty parent string
func TreeErrorEmptyParent() *TreeError {
	return NewTreeError(TreeErrorEmptyString, "parent cannot be empty string")
}

// TreeErrorConflictingParentsValue creates an error for conflicting parents
func TreeErrorConflictingParentsValue(child, parent1, parent2 string) *TreeError {
	return NewTreeError(TreeErrorConflictingParents,
		"child %q has multiple parents: %q, %q (only one allowed)",
		child, parent1, parent2)
}

// TreeErrorUnknownParentValue creates an error for unknown parent
func TreeErrorUnknownParentValue(parent, child string) *TreeError {
	return NewTreeError(TreeErrorUnknownParent,
		"unknown parent %q for child %q. Declare [null, %q] or provide a chain to a root",
		parent, child, parent)
}

// TreeErrorCycleDetected creates an error for cycle detection
func TreeErrorCycleDetected(path []string) *TreeError {
	return NewTreeError(TreeErrorCycle,
		"cycle detected: %s",
		strings.Join(path, " -> "))
}

// TreeErrorCycleNode creates a simple cycle error with just the node name
func TreeErrorCycleNode(node string) *TreeError {
	return NewTreeError(TreeErrorCycle,
		"cycle detected involving node %q",
		node)
}
