// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package scheduler

import (
	"sync"
	"sync/atomic"

	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/run/dag"
)

// Parallel is a parallel scheduler implementing the scheduler.S interface.
type Parallel[V any] struct {
	wg *sync.WaitGroup
	d  *dag.DAG[V]

	state map[dag.ID]*parallelNodeState

	errsMtx sync.Mutex
	errs    errors.List
}

// NewParallel creates a new parallel scheduler for the given DAG.
func NewParallel[V any](d *dag.DAG[V], reverse bool) *Parallel[V] {
	s := &Parallel[V]{
		wg:    &sync.WaitGroup{},
		d:     d,
		state: map[dag.ID]*parallelNodeState{},
	}

	ids := d.IDs()

	// Pass 1 - Create node state
	for _, id := range ids {
		s.state[id] = &parallelNodeState{id: id}
	}

	// Pass 2 - Add forward edges
	if !reverse {
		for id, st := range s.state {
			for _, pid := range d.AncestorsOf(id) {
				pst := s.state[pid]
				pst.successors = append(pst.successors, st)
				st.nRequiredPredecessors++
			}
		}
	} else {
		for id, st := range s.state {
			for _, pid := range d.AncestorsOf(id) {
				pst := s.state[pid]
				st.successors = append(st.successors, pst)
				pst.nRequiredPredecessors++
			}
		}
	}

	return s
}

// Run executes the given function on each node of the DAG.
// Nodes are run in parallel, but no node is visted until all its precessors are done.
func (s *Parallel[V]) Run(f Func[V]) error {
	for _, st := range s.state {
		// Start at root nodes (nodes without any predecessors).
		if st.nRequiredPredecessors == 0 {
			s.visitNode(st, f)
		}
	}

	s.wg.Wait()
	return s.errs.AsError()
}

type parallelNodeState struct {
	id                    dag.ID
	successors            []*parallelNodeState
	nReadyPredecessors    atomic.Int64
	nRequiredPredecessors int64
}

func (s *Parallel[V]) visitNode(st *parallelNodeState, f Func[V]) {
	s.forkTask(func() {
		v, _ := s.d.Node(st.id)

		if err := f(v); err != nil {
			// We collect the errors, but continue. It's up to the caller to cancel the context and skip
			// the function body.
			s.errsMtx.Lock()
			defer s.errsMtx.Unlock()
			s.errs.Append(err)
		}

		st.nReadyPredecessors.Store(0)

		for _, succ := range st.successors {
			// Join paths
			c := succ.nReadyPredecessors.Add(1)
			if c >= succ.nRequiredPredecessors {
				s.visitNode(succ, f)
			}
		}
	})
}

func (s *Parallel[V]) forkTask(body func()) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		body()
	}()
}
