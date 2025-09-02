// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stdlib_test

import (
	stderrors "errors"
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/stdlib"
	"github.com/terramate-io/terramate/test"
)

func TestTmTree(t *testing.T) {
	dir := test.TempDir(t)
	funcs := stdlib.Functions(dir, []string{})
	funcs["tm_tree"] = stdlib.TreeFunc() // Add the function to test it

	t.Run("empty list", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_tree([])`, "test.tm")
		assert.NoError(t, err)
		val, err := evalctx.Eval(expr)
		assert.NoError(t, err)

		paths := val.AsValueSlice()
		assert.EqualInts(t, 0, len(paths))
	})

	t.Run("single root node", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_tree([[null, "root"]])`, "test.tm")
		assert.NoError(t, err)
		val, err := evalctx.Eval(expr)
		assert.NoError(t, err)

		paths := val.AsValueSlice()
		assert.EqualInts(t, 1, len(paths))

		path := paths[0].AsValueSlice()
		assert.EqualInts(t, 1, len(path))
		assert.EqualStrings(t, "root", path[0].AsString())
	})

	t.Run("simple parent-child relationship", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_tree([[null, "root"], ["root", "child"]])`, "test.tm")
		assert.NoError(t, err)
		val, err := evalctx.Eval(expr)
		assert.NoError(t, err)

		paths := val.AsValueSlice()
		assert.EqualInts(t, 2, len(paths))

		// First path should be ["root"]
		path1 := paths[0].AsValueSlice()
		assert.EqualInts(t, 1, len(path1))
		assert.EqualStrings(t, "root", path1[0].AsString())

		// Second path should be ["root", "child"]
		path2 := paths[1].AsValueSlice()
		assert.EqualInts(t, 2, len(path2))
		assert.EqualStrings(t, "root", path2[0].AsString())
		assert.EqualStrings(t, "child", path2[1].AsString())
	})

	t.Run("complex tree with multiple levels", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_tree([
			[null, "root"],
			["root", "child1"],
			["root", "child2"],
			["child1", "grandchild1"],
			["child1", "grandchild2"]
		])`, "test.tm")
		assert.NoError(t, err)
		val, err := evalctx.Eval(expr)
		assert.NoError(t, err)

		paths := val.AsValueSlice()
		assert.EqualInts(t, 5, len(paths))

		// Verify paths are sorted lexicographically
		expectedPaths := [][]string{
			{"root"},
			{"root", "child1"},
			{"root", "child1", "grandchild1"},
			{"root", "child1", "grandchild2"},
			{"root", "child2"},
		}

		for i, expectedPath := range expectedPaths {
			actualPath := paths[i].AsValueSlice()
			assert.EqualInts(t, len(expectedPath), len(actualPath))
			for j, expectedNode := range expectedPath {
				assert.EqualStrings(t, expectedNode, actualPath[j].AsString())
			}
		}
	})

	t.Run("forest with multiple roots", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_tree([
			[null, "root1"],
			[null, "root2"],
			["root1", "child1"],
			["root2", "child2"]
		])`, "test.tm")
		assert.NoError(t, err)
		val, err := evalctx.Eval(expr)
		assert.NoError(t, err)

		paths := val.AsValueSlice()
		assert.EqualInts(t, 4, len(paths))

		// Should be sorted lexicographically
		expectedPaths := [][]string{
			{"root1"},
			{"root1", "child1"},
			{"root2"},
			{"root2", "child2"},
		}

		for i, expectedPath := range expectedPaths {
			actualPath := paths[i].AsValueSlice()
			assert.EqualInts(t, len(expectedPath), len(actualPath))
			for j, expectedNode := range expectedPath {
				assert.EqualStrings(t, expectedNode, actualPath[j].AsString())
			}
		}
	})

	t.Run("unordered input produces sorted output", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_tree([
			["root", "z_child"],
			["root", "a_child"],
			[null, "root"],
			["a_child", "b_grandchild"]
		])`, "test.tm")
		assert.NoError(t, err)
		val, err := evalctx.Eval(expr)
		assert.NoError(t, err)

		paths := val.AsValueSlice()
		assert.EqualInts(t, 4, len(paths))

		expectedPaths := [][]string{
			{"root"},
			{"root", "a_child"},
			{"root", "a_child", "b_grandchild"},
			{"root", "z_child"},
		}

		for i, expectedPath := range expectedPaths {
			actualPath := paths[i].AsValueSlice()
			assert.EqualInts(t, len(expectedPath), len(actualPath))
			for j, expectedNode := range expectedPath {
				assert.EqualStrings(t, expectedNode, actualPath[j].AsString())
			}
		}
	})

	t.Run("error: unknown parent", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_tree([["nonexistent", "child"]])`, "test.tm")
		assert.NoError(t, err)
		_, err = evalctx.Eval(expr)
		assert.Error(t, err)
		assert.IsTrue(t, err.Error() != "")
	})

	t.Run("error: cycle detection", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_tree([
			[null, "a"],
			["a", "b"],
			["b", "c"],
			["c", "a"]
		])`, "test.tm")
		assert.NoError(t, err)
		_, err = evalctx.Eval(expr)
		assert.Error(t, err)
		assert.IsTrue(t, err.Error() != "")
	})

	t.Run("error: conflicting parents", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_tree([
			[null, "root1"],
			[null, "root2"],
			["root1", "child"],
			["root2", "child"]
		])`, "test.tm")
		assert.NoError(t, err)
		_, err = evalctx.Eval(expr)
		assert.Error(t, err)
		assert.IsTrue(t, err.Error() != "")
	})

	t.Run("error: null child", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_tree([[null, null]])`, "test.tm")
		assert.NoError(t, err)
		_, err = evalctx.Eval(expr)
		assert.Error(t, err)
		assert.IsTrue(t, err.Error() != "")
	})

	t.Run("error: empty string child", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_tree([[null, ""]])`, "test.tm")
		assert.NoError(t, err)
		_, err = evalctx.Eval(expr)
		assert.Error(t, err)
		assert.IsTrue(t, err.Error() != "")
	})

	t.Run("error: empty string parent", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_tree([["", "child"]])`, "test.tm")
		assert.NoError(t, err)
		_, err = evalctx.Eval(expr)
		assert.Error(t, err)
		assert.IsTrue(t, err.Error() != "")
	})

	t.Run("self-reference creates cycle error", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_tree([["node", "node"]])`, "test.tm")
		assert.NoError(t, err)
		_, err = evalctx.Eval(expr)
		assert.Error(t, err)
		assert.IsTrue(t, err.Error() != "")
	})

	t.Run("complex forest", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_tree([
			[null, "tree1"],
			[null, "tree2"],
			["tree1", "branch1"],
			["tree1", "branch2"],
			["tree2", "branch3"],
			["branch1", "leaf1"],
			["branch1", "leaf2"],
			["branch3", "leaf3"]
		])`, "test.tm")
		assert.NoError(t, err)
		val, err := evalctx.Eval(expr)
		assert.NoError(t, err)

		paths := val.AsValueSlice()
		assert.EqualInts(t, 8, len(paths))

		expectedPaths := [][]string{
			{"tree1"},
			{"tree1", "branch1"},
			{"tree1", "branch1", "leaf1"},
			{"tree1", "branch1", "leaf2"},
			{"tree1", "branch2"},
			{"tree2"},
			{"tree2", "branch3"},
			{"tree2", "branch3", "leaf3"},
		}

		for i, expectedPath := range expectedPaths {
			actualPath := paths[i].AsValueSlice()
			assert.EqualInts(t, len(expectedPath), len(actualPath))
			for j, expectedNode := range expectedPath {
				assert.EqualStrings(t, expectedNode, actualPath[j].AsString())
			}
		}
	})

	t.Run("single leaf node (no children)", func(t *testing.T) {
		evalctx := eval.NewContext(funcs)
		expr, err := ast.ParseExpression(`tm_tree([[null, "lonely"]])`, "test.tm")
		assert.NoError(t, err)
		val, err := evalctx.Eval(expr)
		assert.NoError(t, err)

		paths := val.AsValueSlice()
		assert.EqualInts(t, 1, len(paths))

		path := paths[0].AsValueSlice()
		assert.EqualInts(t, 1, len(path))
		assert.EqualStrings(t, "lonely", path[0].AsString())
	})
}

func TestTmTreeComprehensiveErrors(t *testing.T) {
	dir := test.TempDir(t)
	funcs := stdlib.Functions(dir, []string{})
	funcs["tm_tree"] = stdlib.TreeFunc()

	testCases := []struct {
		name     string
		input    string
		errorMsg []string // substrings that should be in error message
	}{
		// TreeErrorInvalidInput cases - these are caught by HCL type checking
		{
			name:     "pair with only one element",
			input:    `tm_tree([["single"]])`,
			errorMsg: []string{"tuple required"},
		},
		{
			name:     "pair with three elements",
			input:    `tm_tree([["a", "b", "c"]])`,
			errorMsg: []string{"tuple required"},
		},
		{
			name:     "pair with four elements",
			input:    `tm_tree([["a", "b", "c", "d"]])`,
			errorMsg: []string{"tuple required"},
		},
		{
			name:     "empty pair",
			input:    `tm_tree([[]])`,
			errorMsg: []string{"tuple required"},
		},

		// TreeErrorNullChild cases
		{
			name:     "null child with string parent",
			input:    `tm_tree([["parent", null]])`,
			errorMsg: []string{"child cannot be null"},
		},
		{
			name:     "null child with null parent",
			input:    `tm_tree([[null, null]])`,
			errorMsg: []string{"child cannot be null"},
		},

		// TreeErrorEmptyString cases
		{
			name:     "empty child string",
			input:    `tm_tree([["parent", ""]])`,
			errorMsg: []string{"child cannot be empty"},
		},
		{
			name:     "empty parent string",
			input:    `tm_tree([["", "child"]])`,
			errorMsg: []string{"parent cannot be empty"},
		},
		{
			name:     "empty parent with empty child",
			input:    `tm_tree([["", ""]])`,
			errorMsg: []string{"child cannot be empty"}, // child is checked first
		},

		// TreeErrorUnknownParent cases
		{
			name:     "single unknown parent",
			input:    `tm_tree([["unknown", "child"]])`,
			errorMsg: []string{"unknown parent", "unknown", "child"},
		},
		{
			name:     "chain with unknown root",
			input:    `tm_tree([["unknown", "A"], ["A", "B"], ["B", "C"]])`,
			errorMsg: []string{"unknown parent", "unknown"},
		},
		{
			name:     "multiple disconnected trees with unknown parent",
			input:    `tm_tree([[null, "root1"], ["root1", "child1"], ["unknown", "child2"]])`,
			errorMsg: []string{"unknown parent", "unknown", "child2"},
		},
		{
			name:     "unknown parent in middle of chain",
			input:    `tm_tree([[null, "root"], ["root", "A"], ["B", "C"]])`,
			errorMsg: []string{"unknown parent", "B", "C"},
		},

		// TreeErrorConflictingParents cases
		{
			name:     "child with two different parents",
			input:    `tm_tree([[null, "p1"], [null, "p2"], ["p1", "child"], ["p2", "child"]])`,
			errorMsg: []string{"multiple parents", "child", "p1", "p2"},
		},
		{
			name:     "child with null and non-null parent",
			input:    `tm_tree([[null, "child"], ["parent", "child"]])`,
			errorMsg: []string{"multiple parents", "child", "<root>", "parent"},
		},
		{
			name:     "child with non-null then null parent",
			input:    `tm_tree([["parent", "child"], [null, "child"]])`,
			errorMsg: []string{"multiple parents", "child", "parent", "<root>"},
		},
		{
			name:     "conflict in longer chain",
			input:    `tm_tree([[null, "r1"], [null, "r2"], ["r1", "a"], ["r2", "b"], ["a", "common"], ["b", "common"]])`,
			errorMsg: []string{"multiple parents", "common"},
		},

		// TreeErrorCycle cases
		{
			name:     "simple two-node cycle",
			input:    `tm_tree([[null, "root"], ["root", "A"], ["A", "B"], ["B", "A"]])`,
			errorMsg: []string{"multiple parents", "A"}, // This creates conflicting parents
		},
		{
			name:     "self-reference cycle",
			input:    `tm_tree([["A", "A"]])`,
			errorMsg: []string{"cycle detected", "A"},
		},
		{
			name:     "cycle not involving root",
			input:    `tm_tree([[null, "root"], ["root", "A"], ["A", "B"], ["B", "C"], ["C", "B"]])`,
			errorMsg: []string{"multiple parents", "B"}, // This creates conflicting parents, not a cycle
		},
		{
			name:     "complex cycle in forest",
			input:    `tm_tree([[null, "r1"], [null, "r2"], ["r1", "a"], ["r2", "b"], ["a", "c"], ["c", "a"]])`,
			errorMsg: []string{"multiple parents", "a"}, // This creates conflicting parents, not a cycle
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			evalctx := eval.NewContext(funcs)
			expr, err := ast.ParseExpression(tc.input, "test.tm")
			assert.NoError(t, err, "failed to parse expression")

			_, err = evalctx.Eval(expr)
			assert.Error(t, err, "expected error but got none")
			// Note: Error types are lost through HCL eval layer, so we only check messages

			for _, substring := range tc.errorMsg {
				assert.IsTrue(t, strings.Contains(err.Error(), substring),
					"error message should contain %q but got %q", substring, err.Error())
			}
		})
	}
}

func TestTmTreeErrorTypes(t *testing.T) {
	// Test that errors.IsKind works correctly with direct error values
	t.Run("error type checking", func(t *testing.T) {
		err1 := stdlib.TreeErrorInvalidPairLength()
		assert.IsTrue(t, errors.IsKind(err1, stdlib.TreeErrorInvalidInput))
		assert.IsTrue(t, !errors.IsKind(err1, stdlib.TreeErrorCycle))

		err2 := stdlib.TreeErrorCycleNode("test")
		assert.IsTrue(t, errors.IsKind(err2, stdlib.TreeErrorCycle))
		assert.IsTrue(t, !errors.IsKind(err2, stdlib.TreeErrorInvalidInput))

		// Test with non-TreeError
		normalErr := stderrors.New("some other error")
		assert.IsTrue(t, !errors.IsKind(normalErr, stdlib.TreeErrorCycle))
	})

	t.Run("error messages", func(t *testing.T) {
		// Test all error constructor functions
		err := stdlib.TreeErrorInvalidPairLength()
		assert.IsTrue(t, strings.Contains(err.Error(), "exactly 2 elements"))

		err = stdlib.TreeErrorNullChildValue()
		assert.IsTrue(t, strings.Contains(err.Error(), "child cannot be null"))

		err = stdlib.TreeErrorEmptyChild()
		assert.IsTrue(t, strings.Contains(err.Error(), "child cannot be empty"))

		err = stdlib.TreeErrorEmptyParent()
		assert.IsTrue(t, strings.Contains(err.Error(), "parent cannot be empty"))

		err = stdlib.TreeErrorConflictingParentsValue("child", "parent1", "parent2")
		assert.IsTrue(t, strings.Contains(err.Error(), "child"))
		assert.IsTrue(t, strings.Contains(err.Error(), "parent1"))
		assert.IsTrue(t, strings.Contains(err.Error(), "parent2"))
		assert.IsTrue(t, strings.Contains(err.Error(), "multiple parents"))

		err = stdlib.TreeErrorUnknownParentValue("parent", "child")
		assert.IsTrue(t, strings.Contains(err.Error(), "unknown parent"))
		assert.IsTrue(t, strings.Contains(err.Error(), "parent"))
		assert.IsTrue(t, strings.Contains(err.Error(), "child"))

		err = stdlib.TreeErrorCycleDetected([]string{"A", "B", "C", "A"})
		assert.IsTrue(t, strings.Contains(err.Error(), "cycle detected"))
		assert.IsTrue(t, strings.Contains(err.Error(), "A -> B -> C -> A"))

		err = stdlib.TreeErrorCycleNode("nodeInCycle")
		assert.IsTrue(t, strings.Contains(err.Error(), "cycle detected"))
		assert.IsTrue(t, strings.Contains(err.Error(), "nodeInCycle"))

		// Custom error with formatting
		err = errors.E(stdlib.TreeErrorCycle, "custom message: %s %d", "test", 42)
		assert.IsTrue(t, strings.Contains(err.Error(), "custom message: test 42"))
	})
}
