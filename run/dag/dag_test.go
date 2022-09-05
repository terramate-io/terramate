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

package dag_test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/madlambda/spells/errutil"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/run/dag"

	"github.com/rs/zerolog"
)

type node struct {
	ancestors   []dag.ID
	descendants []dag.ID
}
type testcase struct {
	name   string
	nodes  map[string]node
	err    error
	reason string
	order  []dag.ID
}

func cycleTests() []testcase {
	return []testcase{
		{
			name: "empty dag",
		},
		{
			name: "cycle: A after A",
			nodes: map[string]node{
				"A": {
					ancestors: []dag.ID{"A"},
				},
			},
			err:    errors.E(dag.ErrCycleDetected),
			reason: "A -> A",
		},
		{
			name: "cycle: A after B, B after A",
			nodes: map[string]node{
				"A": {
					ancestors: []dag.ID{"B"},
				},
				"B": {
					ancestors: []dag.ID{"A"},
				},
			},
			err:    errors.E(dag.ErrCycleDetected),
			reason: "A -> B -> A",
		},
		{
			name: "cycle: A after B, B after C, C after A",
			nodes: map[string]node{
				"A": {
					ancestors: []dag.ID{"B"},
				},
				"B": {
					ancestors: []dag.ID{"C"},
				},
				"C": {
					ancestors: []dag.ID{"A"},
				},
			},
			err:    errors.E(dag.ErrCycleDetected),
			reason: "A -> B -> C -> A",
		},
		{
			name: "cycle: A after B, C before B, C after A",
			nodes: map[string]node{
				"A": {
					ancestors: []dag.ID{"B"},
				},
				"C": {
					ancestors:   []dag.ID{"A"},
					descendants: []dag.ID{"B"},
				},
			},
			err:    errors.E(dag.ErrCycleDetected),
			reason: "A -> B -> C -> A",
		},
		{
			name: "cycle: B before A, C before B, C after A",
			nodes: map[string]node{
				"B": {
					descendants: []dag.ID{"A"},
				},
				"C": {
					ancestors:   []dag.ID{"A"},
					descendants: []dag.ID{"B"},
				},
			},
			err:    errors.E(dag.ErrCycleDetected),
			reason: "A -> B -> C -> A",
		},
		{
			name: "cycle: B before A, A after C, C after D, D before B",
			nodes: map[string]node{
				"B": {
					descendants: []dag.ID{"A"},
				},
				"A": {
					ancestors: []dag.ID{"C"},
				},
				"C": {
					ancestors: []dag.ID{"D"},
				},
				"D": {
					descendants: []dag.ID{"B"},
					ancestors:   []dag.ID{"A"},
				},
			},
			err:    errors.E(dag.ErrCycleDetected),
			reason: "A -> B -> D -> A",
		},
		{
			name: "cycle: A after B, B after C, C after D, D after F, F after A",
			nodes: map[string]node{
				"A": {
					ancestors: []dag.ID{"B"},
				},
				"B": {
					ancestors: []dag.ID{"C"},
				},
				"C": {
					ancestors: []dag.ID{"D"},
				},
				"D": {
					ancestors: []dag.ID{"F"},
				},
				"F": {
					ancestors: []dag.ID{"A"},
				},
			},
			err:    errors.E(dag.ErrCycleDetected),
			reason: "A -> B -> C -> D -> F -> A",
		},
		{
			name: "cycle: A after B, B after C, C after D, D after A, F after A",
			nodes: map[string]node{
				"A": {
					ancestors: []dag.ID{"B"},
				},
				"B": {
					ancestors: []dag.ID{"C"},
				},
				"C": {
					ancestors: []dag.ID{"D"},
				},
				"D": {
					ancestors: []dag.ID{"A"},
				},
				"F": {
					ancestors: []dag.ID{"A"},
				},
			},
			err:    errors.E(dag.ErrCycleDetected),
			reason: "A -> B -> C -> D -> A",
		},
		{
			name: "cycle: A after B, B after C, C after D, D after B, F after A",
			nodes: map[string]node{
				"A": {
					ancestors: []dag.ID{"B"},
				},
				"B": {
					ancestors: []dag.ID{"C"},
				},
				"C": {
					ancestors: []dag.ID{"D"},
				},
				"D": {
					ancestors: []dag.ID{"B"},
				},
				"F": {
					ancestors: []dag.ID{"A"},
				},
			},
			err:    errors.E(dag.ErrCycleDetected),
			reason: "A -> B -> C -> D -> B",
		},
		{
			name: "cycle: A before B, B before A",
			nodes: map[string]node{
				"A": {
					descendants: []dag.ID{"B"},
				},
				"B": {
					descendants: []dag.ID{"A"},
				},
			},
			err:    errors.E(dag.ErrCycleDetected),
			reason: "A -> B -> A",
		},
	}
}

func dagTests() []testcase {
	return []testcase{
		{
			name: "simple dag",
			nodes: map[string]node{
				"A": {
					ancestors: []dag.ID{"B"},
				},
				"B": {},
			},
			order: []dag.ID{"B", "A"},
		},
		{
			name: "A -> (B, E), B -> (C, D), D -> E",
			nodes: map[string]node{
				"A": {
					ancestors: []dag.ID{"B", "E"},
				},
				"B": {
					ancestors: []dag.ID{"C", "D"},
				},
				"D": {
					ancestors: []dag.ID{"E"},
				},
				"E": {},
			},
			order: []dag.ID{"C", "E", "D", "B", "A"},
		},
		{
			name: "simple before: A before B",
			nodes: map[string]node{
				"B": {},
				"A": {
					descendants: []dag.ID{"B"},
				},
			},
			order: []dag.ID{"A", "B"},
		},
		{
			name: "A before B, B after C",
			nodes: map[string]node{
				"B": {
					ancestors: []dag.ID{"C"},
				},
				"A": {
					descendants: []dag.ID{"B"},
				},
				"C": {},
			},
			order: []dag.ID{"A", "C", "B"},
		},
		{
			name: "A before B, B before D and after C",
			nodes: map[string]node{
				"A": {
					descendants: []dag.ID{"B"},
				},
				"B": {
					descendants: []dag.ID{"D"},
					ancestors:   []dag.ID{"C"},
				},
				"C": {},
				"D": {},
			},
			order: []dag.ID{"A", "C", "B", "D"},
		},
	}
}

func TestValidatedDAG(t *testing.T) {
	var testcases []testcase
	testcases = append(testcases, cycleTests()...)
	testcases = append(testcases, dagTests()...)

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			d := dag.New()
			var errs []error

			for id, v := range tc.nodes {
				errs = append(errs, d.AddNode(
					dag.ID(id), nil, v.descendants, v.ancestors),
				)
			}

			reason, err := d.Validate()
			if err != nil {
				assert.EqualStrings(t, tc.reason, reason, "cycle reason differ")
				errs = append(errs, err)
			} else {
				order := d.Order()
				assertOrder(t, tc.order, order)
			}

			assert.IsError(t, errutil.Chain(errs...), tc.err, "failed to add node")
		})
	}
}

func assertOrder(t *testing.T, want, got []dag.ID) {
	t.Helper()
	assert.EqualInts(t, len(want), len(got), "length mismatch")
	for i, w := range want {
		assert.EqualStrings(t, string(w), string(got[i]), "id %d mismatch", i)
	}
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
