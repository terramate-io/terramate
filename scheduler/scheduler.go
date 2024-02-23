// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package scheduler

// Func is the function type called when visiting nodes of the DAG.
type Func[V any] func(V) error

// S is the scheduler interface.
type S[V any] interface {
	// Run executes a given function by a specific scheduling strategy.
	Run(Func[V]) error
}
