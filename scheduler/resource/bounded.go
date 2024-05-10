// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package resource

import (
	"context"

	"golang.org/x/sync/semaphore"
)

// Bounded is a bounded resource.
type Bounded struct {
	sem *semaphore.Weighted
}

// NewBounded creates a new bounded resource that can be acquired n times concurrently.
func NewBounded(n int) *Bounded {
	return &Bounded{
		sem: semaphore.NewWeighted(int64(n)),
	}
}

// Acquire acquires the resource. If the resource is already acquired n times,
// wait until another one is released.
func (r *Bounded) Acquire(ctx context.Context) bool {
	err := r.sem.Acquire(ctx, 1)
	return err == nil
}

// Release a previously acquired resource.
func (r *Bounded) Release() {
	r.sem.Release(1)
}
