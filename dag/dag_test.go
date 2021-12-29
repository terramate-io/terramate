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
	"github.com/mineiros-io/terramate/dag"
)

type node struct {
	after  []dag.ID
	before []dag.ID
}
type testcase struct {
	name   string
	nodes  map[string]node
	err    error
	reason string
	order  []dag.ID
}

var cycleTests = []testcase{
	{
		name: "empty dag",
	},
	{
		name: "cycle: A after A",
		nodes: map[string]node{
			"A": {
				after: []dag.ID{"A"},
			},
		},
		err:    dag.ErrCycleDetected,
		reason: "A -> A",
	},
	{
		name: "cycle: A after B, B after A",
		nodes: map[string]node{
			"A": {
				after: []dag.ID{"B"},
			},
			"B": {
				after: []dag.ID{"A"},
			},
		},
		err:    dag.ErrCycleDetected,
		reason: "A -> B -> A",
	},
	{
		name: "cycle: A after B, B after C, C after A",
		nodes: map[string]node{
			"A": {
				after: []dag.ID{"B"},
			},
			"B": {
				after: []dag.ID{"C"},
			},
			"C": {
				after: []dag.ID{"A"},
			},
		},
		err:    dag.ErrCycleDetected,
		reason: "A -> B -> C -> A",
	},
	{
		name: "cycle: A after B, C before B, C after A",
		nodes: map[string]node{
			"A": {
				after: []dag.ID{"B"},
			},
			"C": {
				after:  []dag.ID{"A"},
				before: []dag.ID{"B"},
			},
		},
		err:    dag.ErrCycleDetected,
		reason: "A -> B -> C -> A",
	},
	{
		name: "cycle: B before A, C before B, C after A",
		nodes: map[string]node{
			"B": {
				before: []dag.ID{"A"},
			},
			"C": {
				after:  []dag.ID{"A"},
				before: []dag.ID{"B"},
			},
		},
		err:    dag.ErrCycleDetected,
		reason: "A -> B -> C -> A",
	},
	{
		name: "cycle: B before A, A after C, C after D, D before B",
		nodes: map[string]node{
			"B": {
				before: []dag.ID{"A"},
			},
			"A": {
				after: []dag.ID{"C"},
			},
			"C": {
				after: []dag.ID{"D"},
			},
			"D": {
				before: []dag.ID{"B"},
				after:  []dag.ID{"A"},
			},
		},
		err:    dag.ErrCycleDetected,
		reason: "A -> B -> D -> A",
	},
	{
		name: "cycle: A after B, B after C, C after D, D after F, F after A",
		nodes: map[string]node{
			"A": {
				after: []dag.ID{"B"},
			},
			"B": {
				after: []dag.ID{"C"},
			},
			"C": {
				after: []dag.ID{"D"},
			},
			"D": {
				after: []dag.ID{"F"},
			},
			"F": {
				after: []dag.ID{"A"},
			},
		},
		err:    dag.ErrCycleDetected,
		reason: "A -> B -> C -> D -> F -> A",
	},
	{
		name: "cycle: A after B, B after C, C after D, D after A, F after A",
		nodes: map[string]node{
			"A": {
				after: []dag.ID{"B"},
			},
			"B": {
				after: []dag.ID{"C"},
			},
			"C": {
				after: []dag.ID{"D"},
			},
			"D": {
				after: []dag.ID{"A"},
			},
			"F": {
				after: []dag.ID{"A"},
			},
		},
		err:    dag.ErrCycleDetected,
		reason: "A -> B -> C -> D -> A",
	},
	{
		name: "cycle: A after B, B after C, C after D, D after B, F after A",
		nodes: map[string]node{
			"A": {
				after: []dag.ID{"B"},
			},
			"B": {
				after: []dag.ID{"C"},
			},
			"C": {
				after: []dag.ID{"D"},
			},
			"D": {
				after: []dag.ID{"B"},
			},
			"F": {
				after: []dag.ID{"A"},
			},
		},
		err:    dag.ErrCycleDetected,
		reason: "A -> B -> C -> D -> B",
	},
	{
		name: "cycle: A before B, B before A",
		nodes: map[string]node{
			"A": {
				before: []dag.ID{"B"},
			},
			"B": {
				before: []dag.ID{"A"},
			},
		},
		err:    dag.ErrCycleDetected,
		reason: "A -> B -> A",
	},
}

var dagTests = []testcase{
	{
		name: "simple dag",
		nodes: map[string]node{
			"A": {
				after: []dag.ID{"B"},
			},
			"B": {},
		},
		order: []dag.ID{"B", "A"},
	},
	{
		name: "A -> (B, E), B -> (C, D), D -> E",
		nodes: map[string]node{
			"A": {
				after: []dag.ID{"B", "E"},
			},
			"B": {
				after: []dag.ID{"C", "D"},
			},
			"D": {
				after: []dag.ID{"E"},
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
				before: []dag.ID{"B"},
			},
		},
		order: []dag.ID{"A", "B"},
	},
	{
		name: "A before B, B after C",
		nodes: map[string]node{
			"B": {
				after: []dag.ID{"C"},
			},
			"A": {
				before: []dag.ID{"B"},
			},
			"C": {},
		},
		order: []dag.ID{"A", "C", "B"},
	},
	{
		name: "A before B, B before D and after C",
		nodes: map[string]node{
			"A": {
				before: []dag.ID{"B"},
			},
			"B": {
				before: []dag.ID{"D"},
				after:  []dag.ID{"C"},
			},
			"C": {},
			"D": {},
		},
		order: []dag.ID{"A", "C", "B", "D"},
	},
}

func TestDAG(t *testing.T) {
	var testcases []testcase
	testcases = append(testcases, cycleTests...)
	testcases = append(testcases, dagTests...)

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			d := dag.New()
			var errs []error

			for id, v := range tc.nodes {
				errs = append(errs, d.AddNode(dag.ID(id), nil, v.before, v.after))
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
