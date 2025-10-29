// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package engine_test

import (
	"io"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/rs/zerolog"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/printer"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
	"github.com/terramate-io/terramate/ui/tui/cliconfig"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}

func testEngine(t *testing.T, rootdir string) *engine.Engine {
	t.Helper()
	printers := printer.Printers{
		Stdout: printer.NewPrinter(io.Discard),
		Stderr: printer.NewPrinter(io.Discard),
	}
	e, found, err := engine.Load(rootdir, false, cliconfig.Config{}, engine.HumanMode, printers, 0)
	assert.NoError(t, err)
	assert.IsTrue(t, found, "project not found")
	return e
}

func TestDependencyGraphConstruction(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name   string
		layout []string
		want   map[string][]string // stackPath -> dependencies
	}

	for _, tc := range []testcase{
		{
			// NOTE: stack.after does NOT create data dependencies, only ordering dependencies
			// For testing data dependencies, we need to use input.from_stack_id (tested in TestDependencyGraphWithOutputSharing)
			name: "stack.after does not create data dependencies",
			layout: []string{
				"s:stack-a",
				`s:stack-b:after=["/stack-a"]`,
				`s:stack-c:after=["/stack-b"]`,
			},
			want: map[string][]string{
				"/stack-a": {},
				"/stack-b": {}, // NO dependency on stack-a (after is for ordering only)
				"/stack-c": {}, // NO dependency on stack-b (after is for ordering only)
			},
		},
		{
			name: "multiple after entries do not create dependencies",
			layout: []string{
				"s:stack-a",
				"s:stack-b",
				`s:stack-c:after=["/stack-a", "/stack-b"]`,
			},
			want: map[string][]string{
				"/stack-a": {},
				"/stack-b": {},
				"/stack-c": {}, // NO dependencies (after is for ordering only)
			},
		},
		{
			name: "no dependencies",
			layout: []string{
				"s:stack-a",
				"s:stack-b",
				"s:stack-c",
			},
			want: map[string][]string{
				"/stack-a": {},
				"/stack-b": {},
				"/stack-c": {},
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := sandbox.New(t)
			s.BuildTree(tc.layout)
			s.Git().CommitAll("initial commit")

			e := testEngine(t, s.RootDir())

			allStacks, err := config.LoadAllStacks(e.Config(), e.Config().Tree())
			assert.NoError(t, err)

			graph, err := e.NewDependencyGraph(allStacks, nil, "")
			assert.NoError(t, err)

			// Verify dependencies
			for stackPath, expectedDeps := range tc.want {
				actualDeps := graph.GetDirectDependencies(stackPath)
				assert.EqualInts(t, len(expectedDeps), len(actualDeps),
					"stack %s: expected %d dependencies, got %d", stackPath, len(expectedDeps), len(actualDeps))

				for _, expectedDep := range expectedDeps {
					found := false
					for _, actualDep := range actualDeps {
						if actualDep == expectedDep {
							found = true
							break
						}
					}
					assert.IsTrue(t, found, "stack %s: missing expected dependency %s", stackPath, expectedDep)
				}
			}
		})
	}
}

func TestDependencyGraphTransitiveDependencies(t *testing.T) {
	t.Parallel()
	s := sandbox.New(t)
	s.BuildTree([]string{
		`f:terramate.tm:` + Terramate(
			Config(
				Experiments(hcl.SharingIsCaringExperimentName),
			),
		).String(),
		`f:sharing.tm:` + Block("sharing_backend",
			Labels("default"),
			Expr("type", "terraform"),
			Command("terraform", "output", "-json"),
			Str("filename", "_sharing.tf"),
		).String(),
		"s:stack-a:id=stack-a",
		"s:stack-b:id=stack-b",
		"s:stack-c:id=stack-c",
		"s:stack-d:id=stack-d",
	})

	// Set up output sharing: stack-b -> stack-a, stack-c -> stack-b, stack-d -> stack-c
	s.RootEntry().CreateFile("stack-a/outputs.tm", Block("output",
		Labels("out_a"),
		Str("backend", "default"),
		Expr("value", "a"),
	).String())

	s.RootEntry().CreateFile("stack-b/inputs.tm", Block("input",
		Labels("in_a"),
		Str("backend", "default"),
		Expr("value", "outputs.out_a.value"),
		Str("from_stack_id", "stack-a"),
	).String())
	s.RootEntry().CreateFile("stack-b/outputs.tm", Block("output",
		Labels("out_b"),
		Str("backend", "default"),
		Expr("value", "b"),
	).String())

	s.RootEntry().CreateFile("stack-c/inputs.tm", Block("input",
		Labels("in_b"),
		Str("backend", "default"),
		Expr("value", "outputs.out_b.value"),
		Str("from_stack_id", "stack-b"),
	).String())
	s.RootEntry().CreateFile("stack-c/outputs.tm", Block("output",
		Labels("out_c"),
		Str("backend", "default"),
		Expr("value", "c"),
	).String())

	s.RootEntry().CreateFile("stack-d/inputs.tm", Block("input",
		Labels("in_c"),
		Str("backend", "default"),
		Expr("value", "outputs.out_c.value"),
		Str("from_stack_id", "stack-c"),
	).String())

	s.Git().CommitAll("initial commit")

	e := testEngine(t, s.RootDir())

	allStacks, err := config.LoadAllStacks(e.Config(), e.Config().Tree())
	assert.NoError(t, err)

	graph, err := e.NewDependencyGraph(allStacks, nil, "")
	assert.NoError(t, err)

	// Test transitive dependencies for stack-d
	allDeps := graph.GetAllDependencies("/stack-d")
	assert.EqualInts(t, 3, len(allDeps), "expected 3 transitive dependencies")

	expected := map[string]bool{
		"/stack-a": true,
		"/stack-b": true,
		"/stack-c": true,
	}
	for _, dep := range allDeps {
		assert.IsTrue(t, expected[dep], "unexpected dependency: %s", dep)
	}
}

func TestDependencyGraphDependents(t *testing.T) {
	t.Parallel()
	s := sandbox.New(t)
	s.BuildTree([]string{
		`f:terramate.tm:` + Terramate(
			Config(
				Experiments(hcl.SharingIsCaringExperimentName),
			),
		).String(),
		`f:sharing.tm:` + Block("sharing_backend",
			Labels("default"),
			Expr("type", "terraform"),
			Command("terraform", "output", "-json"),
			Str("filename", "_sharing.tf"),
		).String(),
		"s:stack-a:id=stack-a",
		"s:stack-b:id=stack-b",
		"s:stack-c:id=stack-c",
		"s:stack-d:id=stack-d",
	})

	// Set up output sharing: stack-b -> stack-a, stack-c -> stack-a, stack-d -> stack-b
	s.RootEntry().CreateFile("stack-a/outputs.tm", Block("output",
		Labels("out_a"),
		Str("backend", "default"),
		Expr("value", "a"),
	).String())

	s.RootEntry().CreateFile("stack-b/inputs.tm", Block("input",
		Labels("in_a"),
		Str("backend", "default"),
		Expr("value", "outputs.out_a.value"),
		Str("from_stack_id", "stack-a"),
	).String())
	s.RootEntry().CreateFile("stack-b/outputs.tm", Block("output",
		Labels("out_b"),
		Str("backend", "default"),
		Expr("value", "b"),
	).String())

	s.RootEntry().CreateFile("stack-c/inputs.tm", Block("input",
		Labels("in_a2"),
		Str("backend", "default"),
		Expr("value", "outputs.out_a.value"),
		Str("from_stack_id", "stack-a"),
	).String())

	s.RootEntry().CreateFile("stack-d/inputs.tm", Block("input",
		Labels("in_b"),
		Str("backend", "default"),
		Expr("value", "outputs.out_b.value"),
		Str("from_stack_id", "stack-b"),
	).String())

	s.Git().CommitAll("initial commit")

	e := testEngine(t, s.RootDir())

	allStacks, err := config.LoadAllStacks(e.Config(), e.Config().Tree())
	assert.NoError(t, err)

	graph, err := e.NewDependencyGraph(allStacks, nil, "")
	assert.NoError(t, err)

	// Test direct dependents of stack-a
	directDependents := graph.GetDirectDependents("/stack-a")
	assert.EqualInts(t, 2, len(directDependents), "expected 2 direct dependents of stack-a")

	expected := map[string]bool{
		"/stack-b": true,
		"/stack-c": true,
	}
	for _, dep := range directDependents {
		assert.IsTrue(t, expected[dep], "unexpected dependent: %s", dep)
	}

	// Test all dependents (transitive) of stack-a
	allDependents := graph.GetAllDependents("/stack-a")
	assert.EqualInts(t, 3, len(allDependents), "expected 3 transitive dependents of stack-a")

	expectedAll := map[string]bool{
		"/stack-b": true,
		"/stack-c": true,
		"/stack-d": true, // transitive through stack-b
	}
	for _, dep := range allDependents {
		assert.IsTrue(t, expectedAll[dep], "unexpected dependent: %s", dep)
	}
}

func TestDependencyGraphWithOutputSharing(t *testing.T) {
	t.Parallel()
	s := sandbox.New(t)
	s.BuildTree([]string{
		`f:terramate.tm:` + Terramate(
			Config(
				Experiments(hcl.SharingIsCaringExperimentName),
			),
		).String(),
		`f:sharing.tm:` + Block("sharing_backend",
			Labels("default"),
			Expr("type", "terraform"),
			Command("terraform", "output", "-json"),
			Str("filename", "_sharing.tf"),
		).String(),
		"s:stack-a:id=stack-a",
		"s:stack-b:id=stack-b",
	})

	// Add output to stack-a
	s.RootEntry().CreateFile("stack-a/outputs.tm", Block("output",
		Labels("myoutput"),
		Str("backend", "default"),
		Expr("value", "some.value"),
	).String())

	// Add input to stack-b that depends on stack-a
	s.RootEntry().CreateFile("stack-b/inputs.tm", Block("input",
		Labels("myinput"),
		Str("backend", "default"),
		Expr("value", "outputs.myoutput.value"),
		Str("from_stack_id", "stack-a"),
	).String())

	s.Git().CommitAll("initial commit")

	e := testEngine(t, s.RootDir())

	allStacks, err := config.LoadAllStacks(e.Config(), e.Config().Tree())
	assert.NoError(t, err)

	graph, err := e.NewDependencyGraph(allStacks, nil, "")
	assert.NoError(t, err)

	// Verify stack-b depends on stack-a via input.from_stack_id
	deps := graph.GetDirectDependencies("/stack-b")
	assert.EqualInts(t, 1, len(deps), "expected 1 dependency")
	assert.EqualStrings(t, "/stack-a", deps[0])

	// Verify stack-a is a dependent of stack-b (inverse relationship)
	dependents := graph.GetDirectDependents("/stack-a")
	assert.EqualInts(t, 1, len(dependents), "expected 1 dependent")
	assert.EqualStrings(t, "/stack-b", dependents[0])
}

func TestApplyDependencyFilters(t *testing.T) {
	t.Parallel()

	// Common setup for sharing experiment
	sharingSetup := []string{
		`f:terramate.tm:` + Terramate(
			Config(
				Experiments(hcl.SharingIsCaringExperimentName),
			),
		).String(),
		`f:sharing.tm:` + Block("sharing_backend",
			Labels("default"),
			Expr("type", "terraform"),
			Command("terraform", "output", "-json"),
			Str("filename", "_sharing.tf"),
		).String(),
	}

	type testcase struct {
		name           string
		layout         []string
		setupFunc      func(sandbox.S) // function to set up inputs/outputs
		filterOpts     engine.DependencyFilters
		initialStacks  []string // paths of initially selected stacks
		expectedStacks []string // paths of expected result stacks
	}

	for _, tc := range []testcase{
		{
			name: "only-dependencies: replace with dependencies",
			layout: append(sharingSetup, []string{
				"s:stack-a:id=stack-a",
				"s:stack-b:id=stack-b",
				"s:stack-c:id=stack-c",
			}...),
			setupFunc: func(s sandbox.S) {
				// stack-b depends on stack-a, stack-c depends on stack-b
				s.RootEntry().CreateFile("stack-a/outputs.tm", Block("output",
					Labels("out_a"),
					Str("backend", "default"),
					Expr("value", "a"),
				).String())
				s.RootEntry().CreateFile("stack-b/inputs.tm", Block("input",
					Labels("in_a"),
					Str("backend", "default"),
					Expr("value", "outputs.out_a.value"),
					Str("from_stack_id", "stack-a"),
				).String())
				s.RootEntry().CreateFile("stack-b/outputs.tm", Block("output",
					Labels("out_b"),
					Str("backend", "default"),
					Expr("value", "b"),
				).String())
				s.RootEntry().CreateFile("stack-c/inputs.tm", Block("input",
					Labels("in_b"),
					Str("backend", "default"),
					Expr("value", "outputs.out_b.value"),
					Str("from_stack_id", "stack-b"),
				).String())
			},
			filterOpts: engine.DependencyFilters{
				OnlyAllDependencies: true,
			},
			initialStacks:  []string{"/stack-c"},
			expectedStacks: []string{"/stack-a", "/stack-b"},
		},
		{
			name: "include-dependencies: add dependencies to selection",
			layout: append(sharingSetup, []string{
				"s:stack-a:id=stack-a",
				"s:stack-b:id=stack-b",
				"s:stack-c:id=stack-c",
			}...),
			setupFunc: func(s sandbox.S) {
				s.RootEntry().CreateFile("stack-a/outputs.tm", Block("output",
					Labels("out_a"),
					Str("backend", "default"),
					Expr("value", "a"),
				).String())
				s.RootEntry().CreateFile("stack-b/inputs.tm", Block("input",
					Labels("in_a"),
					Str("backend", "default"),
					Expr("value", "outputs.out_a.value"),
					Str("from_stack_id", "stack-a"),
				).String())
				s.RootEntry().CreateFile("stack-b/outputs.tm", Block("output",
					Labels("out_b"),
					Str("backend", "default"),
					Expr("value", "b"),
				).String())
				s.RootEntry().CreateFile("stack-c/inputs.tm", Block("input",
					Labels("in_b"),
					Str("backend", "default"),
					Expr("value", "outputs.out_b.value"),
					Str("from_stack_id", "stack-b"),
				).String())
			},
			filterOpts: engine.DependencyFilters{
				IncludeAllDependencies: true,
			},
			initialStacks:  []string{"/stack-c"},
			expectedStacks: []string{"/stack-a", "/stack-b", "/stack-c"},
		},
		{
			name: "only-direct-dependents: replace with direct dependents",
			layout: append(sharingSetup, []string{
				"s:stack-a:id=stack-a",
				"s:stack-b:id=stack-b",
				"s:stack-c:id=stack-c",
			}...),
			setupFunc: func(s sandbox.S) {
				s.RootEntry().CreateFile("stack-a/outputs.tm", Block("output",
					Labels("out_a"),
					Str("backend", "default"),
					Expr("value", "a"),
				).String())
				s.RootEntry().CreateFile("stack-b/inputs.tm", Block("input",
					Labels("in_a"),
					Str("backend", "default"),
					Expr("value", "outputs.out_a.value"),
					Str("from_stack_id", "stack-a"),
				).String())
				s.RootEntry().CreateFile("stack-b/outputs.tm", Block("output",
					Labels("out_b"),
					Str("backend", "default"),
					Expr("value", "b"),
				).String())
				s.RootEntry().CreateFile("stack-c/inputs.tm", Block("input",
					Labels("in_b"),
					Str("backend", "default"),
					Expr("value", "outputs.out_b.value"),
					Str("from_stack_id", "stack-b"),
				).String())
			},
			filterOpts: engine.DependencyFilters{
				OnlyDirectDependents: true,
			},
			initialStacks:  []string{"/stack-a"},
			expectedStacks: []string{"/stack-b"},
		},
		{
			name: "only-all-dependents: replace with all dependents",
			layout: append(sharingSetup, []string{
				"s:stack-a:id=stack-a",
				"s:stack-b:id=stack-b",
				"s:stack-c:id=stack-c",
			}...),
			setupFunc: func(s sandbox.S) {
				s.RootEntry().CreateFile("stack-a/outputs.tm", Block("output",
					Labels("out_a"),
					Str("backend", "default"),
					Expr("value", "a"),
				).String())
				s.RootEntry().CreateFile("stack-b/inputs.tm", Block("input",
					Labels("in_a"),
					Str("backend", "default"),
					Expr("value", "outputs.out_a.value"),
					Str("from_stack_id", "stack-a"),
				).String())
				s.RootEntry().CreateFile("stack-b/outputs.tm", Block("output",
					Labels("out_b"),
					Str("backend", "default"),
					Expr("value", "b"),
				).String())
				s.RootEntry().CreateFile("stack-c/inputs.tm", Block("input",
					Labels("in_b"),
					Str("backend", "default"),
					Expr("value", "outputs.out_b.value"),
					Str("from_stack_id", "stack-b"),
				).String())
			},
			filterOpts: engine.DependencyFilters{
				OnlyAllDependents: true,
			},
			initialStacks:  []string{"/stack-a"},
			expectedStacks: []string{"/stack-b", "/stack-c"},
		},
		{
			name: "include-all-dependents: add all dependents to selection",
			layout: append(sharingSetup, []string{
				"s:stack-a:id=stack-a",
				"s:stack-b:id=stack-b",
				"s:stack-c:id=stack-c",
			}...),
			setupFunc: func(s sandbox.S) {
				s.RootEntry().CreateFile("stack-a/outputs.tm", Block("output",
					Labels("out_a"),
					Str("backend", "default"),
					Expr("value", "a"),
				).String())
				s.RootEntry().CreateFile("stack-b/inputs.tm", Block("input",
					Labels("in_a"),
					Str("backend", "default"),
					Expr("value", "outputs.out_a.value"),
					Str("from_stack_id", "stack-a"),
				).String())
				s.RootEntry().CreateFile("stack-b/outputs.tm", Block("output",
					Labels("out_b"),
					Str("backend", "default"),
					Expr("value", "b"),
				).String())
				s.RootEntry().CreateFile("stack-c/inputs.tm", Block("input",
					Labels("in_b"),
					Str("backend", "default"),
					Expr("value", "outputs.out_b.value"),
					Str("from_stack_id", "stack-b"),
				).String())
			},
			filterOpts: engine.DependencyFilters{
				IncludeAllDependents: true,
			},
			initialStacks:  []string{"/stack-a"},
			expectedStacks: []string{"/stack-a", "/stack-b", "/stack-c"},
		},
		{
			name: "include-direct-dependents: add direct dependents only",
			layout: append(sharingSetup, []string{
				"s:stack-a:id=stack-a",
				"s:stack-b:id=stack-b",
				"s:stack-c:id=stack-c",
			}...),
			setupFunc: func(s sandbox.S) {
				s.RootEntry().CreateFile("stack-a/outputs.tm", Block("output",
					Labels("out_a"),
					Str("backend", "default"),
					Expr("value", "a"),
				).String())
				s.RootEntry().CreateFile("stack-b/inputs.tm", Block("input",
					Labels("in_a"),
					Str("backend", "default"),
					Expr("value", "outputs.out_a.value"),
					Str("from_stack_id", "stack-a"),
				).String())
				s.RootEntry().CreateFile("stack-b/outputs.tm", Block("output",
					Labels("out_b"),
					Str("backend", "default"),
					Expr("value", "b"),
				).String())
				s.RootEntry().CreateFile("stack-c/inputs.tm", Block("input",
					Labels("in_b"),
					Str("backend", "default"),
					Expr("value", "outputs.out_b.value"),
					Str("from_stack_id", "stack-b"),
				).String())
			},
			filterOpts: engine.DependencyFilters{
				IncludeDirectDependents: true,
			},
			initialStacks:  []string{"/stack-a"},
			expectedStacks: []string{"/stack-a", "/stack-b"},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := sandbox.New(t)
			s.BuildTree(tc.layout)

			// Call setup function if provided
			if tc.setupFunc != nil {
				tc.setupFunc(s)
			}

			s.Git().CommitAll("initial commit")

			e := testEngine(t, s.RootDir())

			allStacks, err := config.LoadAllStacks(e.Config(), e.Config().Tree())
			assert.NoError(t, err)

			// Build initial stack selection
			initialSelection := config.List[*config.SortableStack]{}
			for _, stackPath := range tc.initialStacks {
				for _, st := range allStacks {
					if st.Stack.Dir.String() == stackPath {
						initialSelection = append(initialSelection, st)
						break
					}
				}
			}

			// Apply filters
			result, err := e.ApplyDependencyFilters(tc.filterOpts, initialSelection, "")
			assert.NoError(t, err)

			// Verify result
			resultPaths := make(map[string]bool)
			for _, st := range result {
				resultPaths[st.Stack.Dir.String()] = true
			}

			assert.EqualInts(t, len(tc.expectedStacks), len(result),
				"expected %d stacks, got %d", len(tc.expectedStacks), len(result))

			for _, expected := range tc.expectedStacks {
				assert.IsTrue(t, resultPaths[expected],
					"expected stack %s not found in result", expected)
			}
		})
	}
}

func TestDeprecatedFlags(t *testing.T) {
	t.Parallel()
	s := sandbox.New(t)
	s.BuildTree([]string{
		`f:terramate.tm:` + Terramate(
			Config(
				Experiments(hcl.SharingIsCaringExperimentName),
			),
		).String(),
		"s:stack-a",
		`s:stack-b:after=["/stack-a"]`,
	})
	s.Git().CommitAll("initial commit")

	e := testEngine(t, s.RootDir())

	allStacks, err := config.LoadAllStacks(e.Config(), e.Config().Tree())
	assert.NoError(t, err)

	// Select stack-b
	initialSelection := config.List[*config.SortableStack]{}
	for _, st := range allStacks {
		if st.Stack.Dir.String() == "/stack-b" {
			initialSelection = append(initialSelection, st)
			break
		}
	}

	// Test deprecated IncludeOutputDependencies flag
	result, err := e.ApplyDependencyFilters(engine.DependencyFilters{
		IncludeOutputDependencies: true,
	}, initialSelection, "")
	assert.NoError(t, err)

	// Should only include stack-b (no output dependencies found)
	assert.EqualInts(t, 1, len(result))

	resultPaths := make(map[string]bool)
	for _, st := range result {
		resultPaths[st.Stack.Dir.String()] = true
	}
	assert.IsTrue(t, resultPaths["/stack-b"])
}
