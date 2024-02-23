// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package scheduler

import (
	"slices"

	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/run/dag"
)

// Sequential is a sequential scheduler implementing the scheduler.S interface.
type Sequential[V any] struct {
	d       *dag.DAG[V]
	reverse bool
}

// NewSequential creates a new sequential scheduler for the given DAG.
func NewSequential[V any](d *dag.DAG[V], reverse bool) *Sequential[V] {
	return &Sequential[V]{d: d, reverse: reverse}
}

// Run executes the given function on each node of the DAG.
// Nodes are run sequentially by their topological order.
func (s *Sequential[V]) Run(f Func[V]) error {
	errs := errors.L()

	order := s.d.Order()
	if s.reverse {
		slices.Reverse(order)
	}

	for _, id := range order {
		v, _ := s.d.Node(id)
		errs.Append(f(v))
	}

	return errs.AsError()
}
