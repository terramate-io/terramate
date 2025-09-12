// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stdlib

import (
	"strings"

	"github.com/terramate-io/terramate/errors"
)

// TreeError kinds for tm_tree function errors
const (
	TreeErrorInvalidInput       errors.Kind = "tree: invalid input"
	TreeErrorConflictingParents errors.Kind = "tree: conflicting parents"
	TreeErrorUnknownParent      errors.Kind = "tree: unknown parent"
	TreeErrorCycle              errors.Kind = "tree: cycle detected"
	TreeErrorNullChild          errors.Kind = "tree: null child"
	TreeErrorEmptyString        errors.Kind = "tree: empty string"
)

// TreeErrorInvalidPairLength creates an error for invalid pair length
func TreeErrorInvalidPairLength() error {
	return errors.E(TreeErrorInvalidInput, "each pair must have exactly 2 elements")
}

// TreeErrorNullChildValue creates an error for null child value
func TreeErrorNullChildValue() error {
	return errors.E(TreeErrorNullChild, "child cannot be null")
}

// TreeErrorEmptyChild creates an error for empty child string
func TreeErrorEmptyChild() error {
	return errors.E(TreeErrorEmptyString, "child cannot be empty string")
}

// TreeErrorEmptyParent creates an error for empty parent string
func TreeErrorEmptyParent() error {
	return errors.E(TreeErrorEmptyString, "parent cannot be empty string")
}

// TreeErrorConflictingParentsValue creates an error for conflicting parents
func TreeErrorConflictingParentsValue(child, parent1, parent2 string) error {
	return errors.E(TreeErrorConflictingParents,
		"child %q has multiple parents: %q, %q (only one allowed)",
		child, parent1, parent2)
}

// TreeErrorUnknownParentValue creates an error for unknown parent
func TreeErrorUnknownParentValue(parent, child string) error {
	return errors.E(TreeErrorUnknownParent,
		"unknown parent %q for child %q. Declare [null, %q] or provide a chain to a root",
		parent, child, parent)
}

// TreeErrorCycleDetected creates an error for cycle detection
func TreeErrorCycleDetected(path []string) error {
	return errors.E(TreeErrorCycle,
		"cycle detected: %s",
		strings.Join(path, " -> "))
}

// TreeErrorCycleNode creates a simple cycle error with just the node name
func TreeErrorCycleNode(node string) error {
	return errors.E(TreeErrorCycle,
		"cycle detected involving node %q",
		node)
}
