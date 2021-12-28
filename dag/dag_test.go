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

type vertice struct {
	id     dag.ID
	after  []dag.ID
	before []dag.ID
}
type testcase struct {
	name     string
	vertices []vertice
	err      error
	reason   string
	order    []dag.ID
}

var cycleTests = []testcase{
	{
		name: "empty dag",
	},
	{
		name: "simple cycle",
		vertices: []vertice{
			{
				id:    "A",
				after: []dag.ID{"A"},
			},
		},
		err:    dag.ErrCycleDetected,
		reason: "A -> A",
	},
	{
		name: "cycle: A -> B, B -> A",
		vertices: []vertice{
			{
				id:    "A",
				after: []dag.ID{"B"},
			},
			{
				id:    "B",
				after: []dag.ID{"A"},
			},
		},
		err:    dag.ErrCycleDetected,
		reason: "A -> B -> A",
	},
	{
		name: "after cycle: A -> B, B -> C, C -> A",
		vertices: []vertice{
			{
				id:    "A",
				after: []dag.ID{"B"},
			},
			{
				id:    "B",
				after: []dag.ID{"C"},
			},
			{
				id:    "C",
				after: []dag.ID{"A"},
			},
		},
		err:    dag.ErrCycleDetected,
		reason: "A -> B -> C -> A",
	},
	{
		name: "after/before cycle: A after B, C before B, C after A",
		vertices: []vertice{
			{
				id:    "A",
				after: []dag.ID{"B"},
			},
			{
				id:     "C",
				after:  []dag.ID{"A"},
				before: []dag.ID{"B"},
			},
		},
		err:    dag.ErrCycleDetected,
		reason: "A -> B -> C -> A",
	},
	{
		name: "after cycle: A -> B, B -> C, C -> D, D -> F, F -> A",
		vertices: []vertice{
			{
				id:    "A",
				after: []dag.ID{"B"},
			},
			{
				id:    "B",
				after: []dag.ID{"C"},
			},
			{
				id:    "C",
				after: []dag.ID{"D"},
			},
			{
				id:    "D",
				after: []dag.ID{"F"},
			},
			{
				id:    "F",
				after: []dag.ID{"A"},
			},
		},
		err:    dag.ErrCycleDetected,
		reason: "A -> B -> C -> D -> F -> A",
	},
	{
		name: "cycle: A -> B, B -> C, C -> D, D -> A, F -> A",
		vertices: []vertice{
			{
				id:    "A",
				after: []dag.ID{"B"},
			},
			{
				id:    "B",
				after: []dag.ID{"C"},
			},
			{
				id:    "C",
				after: []dag.ID{"D"},
			},
			{
				id:    "D",
				after: []dag.ID{"A"},
			},
			{
				id:    "F",
				after: []dag.ID{"A"},
			},
		},
		err:    dag.ErrCycleDetected,
		reason: "A -> B -> C -> D -> A",
	},
}

var dagTests = []testcase{
	{
		name: "simple dag",
		vertices: []vertice{
			{
				id:    "A",
				after: []dag.ID{"B"},
			},
			{
				id: "B",
			},
		},
		order: []dag.ID{"B", "A"},
	},
	{
		name: "A -> (B, E), B -> (C, D), D -> E",
		vertices: []vertice{
			{
				id:    "A",
				after: []dag.ID{"B", "E"},
			},
			{
				id:    "B",
				after: []dag.ID{"C", "D"},
			},
			{
				id:    "D",
				after: []dag.ID{"E"},
			},
			{
				id: "E",
			},
		},
		order: []dag.ID{"C", "E", "D", "B", "A"},
	},
	{
		name: "simple before: A before B",
		vertices: []vertice{
			{
				id: "B",
			},
			{
				id:     "A",
				before: []dag.ID{"B"},
			},
		},
		order: []dag.ID{"A", "B"},
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
			for _, v := range tc.vertices {
				errs = append(errs, d.AddVertice(v.id, nil, v.before, v.after))
			}
			reason, err := d.Validate()
			if err != nil {
				assert.EqualStrings(t, tc.reason, reason, "cycle reason differ")
				errs = append(errs, err)
			} else {
				order := d.Order()
				assertOrder(t, tc.order, order)
			}

			assert.IsError(t, errutil.Chain(errs...), tc.err, "failed to add vertice")

		})
	}
}

func TestBeforeAfterVerticeDAG(t *testing.T) {
	for _, tc := range []testcase{
		{
			name: "1",
			vertices: []vertice{
				{
					id: "B",
				},
				{
					id:     "A",
					before: []dag.ID{"B"},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := dag.New()
			var errs []error
			for _, v := range tc.vertices {
				errs = append(errs, d.AddVertice(v.id, nil, v.before, v.after))
			}
			assert.IsError(t, errutil.Chain(errs...), tc.err, "failed to add vertice")
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
