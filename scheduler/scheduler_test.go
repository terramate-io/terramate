// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package scheduler_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/run/dag"
	"github.com/terramate-io/terramate/scheduler"
	"github.com/terramate-io/terramate/scheduler/resource"
)

func TestSimpleSequential(t *testing.T) {
	t.Parallel()

	r := resource.NewBounded(5)
	g := scheduler.NewSequential(makeDAG(), false)
	ctx := context.Background()

	err := g.Run(func(s string) error {
		r.Acquire(ctx)
		defer r.Release()
		fmt.Println(s)
		return nil
	})
	assert.NoError(t, err)
}

func TestSimpleParallel(t *testing.T) {
	t.Parallel()

	r := resource.NewBounded(1)
	g := scheduler.NewParallel(makeDAG(), false)
	ctx := context.Background()

	err := g.Run(func(s string) error {
		r.Acquire(ctx)
		defer r.Release()
		fmt.Println(s)
		return nil
	})
	assert.NoError(t, err)
}

func makeDAG() *dag.DAG[string] {
	d := dag.New[string]()

	addNode := func(s string, preds []dag.ID) {
		_ = d.AddNode(
			dag.ID(s),
			s,
			nil,
			preds,
		)
	}

	addNode("z", []dag.ID{"a", "b", "c"})
	addNode("a", nil)
	addNode("a/1", []dag.ID{"a"})
	addNode("a/2", []dag.ID{"a", "a/1"})
	addNode("a/3", []dag.ID{"a", "a/2"})
	addNode("b", nil)
	addNode("b/1", []dag.ID{"b"})
	addNode("b/2", []dag.ID{"b"})
	addNode("b/3", []dag.ID{"b"})
	addNode("c", nil)
	return d
}

type gridNode struct {
	idx  int
	aIdx int
	bIdx int
}

// makeGridDAG builds a DAG for an NxN matrix where A[i][j] has predecessors A[i-1][j] A[i][j-1],
// unless the respective indices are out of bounds (i.e. for first row and column).
func makeGridDAG() *dag.DAG[gridNode] {
	d := dag.New[gridNode]()

	addNode := func(s string, nd gridNode, preds []dag.ID) {
		_ = d.AddNode(dag.ID(s), nd, nil, preds)
	}

	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			var preds []dag.ID
			id := fmt.Sprintf("%v.%v", i, j)
			nd := gridNode{idx: i*10 + j}

			if i > 0 {
				preds = append(preds, dag.ID(fmt.Sprintf("%v.%v", i-1, j)))
				nd.aIdx = (i-1)*10 + j
			}

			if j > 0 {
				preds = append(preds, dag.ID(fmt.Sprintf("%v.%v", i, j-1)))
				nd.bIdx = i*10 + (j - 1)
			}

			addNode(id, nd, preds)
		}

	}

	return d
}

func TestSequentialGrid(t *testing.T) {
	t.Parallel()

	d := makeGridDAG()
	ndarr := make([]int, 10*10)

	g := scheduler.NewSequential(d, false)

	err := g.Run(func(nd gridNode) error {
		v := 1

		if nd.aIdx != -1 {
			v += ndarr[nd.aIdx]
		}

		if nd.bIdx != -1 {
			v += ndarr[nd.bIdx]
		}

		ndarr[nd.idx] = v
		return nil
	})

	assert.NoError(t, err)
	assert.EqualInts(t, 272271, ndarr[99], "invalid result")
}

func TestParallelGrid(t *testing.T) {
	t.Parallel()

	d := makeGridDAG()
	ndarr := make([]int, 10*10)

	g := scheduler.NewParallel(d, false)

	err := g.Run(func(nd gridNode) error {
		v := 1

		if nd.aIdx != -1 {
			v += ndarr[nd.aIdx]
		}

		if nd.bIdx != -1 {
			v += ndarr[nd.bIdx]
		}

		ndarr[nd.idx] = v
		return nil
	})

	// We have a DAG with lots of ordering constraints, then we incrementally compute values that depend on already
	// computed predecessor values. If the parallel processing violates order, we will end up with a wrong result.
	assert.NoError(t, err)
	assert.EqualInts(t, 272271, ndarr[99], "invalid result")
}
