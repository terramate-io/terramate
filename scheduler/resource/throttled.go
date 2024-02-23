// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package resource

import (
	"context"
	"time"
)

// Throttled in a rate-limited resource.
type Throttled struct {
	ticker *time.Ticker
}

// NewThrottled creates a resource with the given rate limit.
func NewThrottled(requestsPerSecond int64) *Throttled {
	interval := time.Duration(1.0/float64(requestsPerSecond)*1000.0) * time.Millisecond
	return &Throttled{
		ticker: time.NewTicker(interval),
	}
}

// Acquire can be called concurrently, and blocks callers so that are let through as
// the rate limit requires.
func (r *Throttled) Acquire(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case <-r.ticker.C:
		return true
	}
}

// Release for this resource type is no-op.
func (*Throttled) Release() {
}
