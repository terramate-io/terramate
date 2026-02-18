// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package preempt implements cooperative scheduling for preemptable functions
// that can await keys produced by other functions.
package preempt

import (
	"context"
	"iter"

	"github.com/terramate-io/terramate/errors"
)

// ErrUnresolvable is an error that indicates that [Await] could not complete.
const ErrUnresolvable errors.Kind = "unresolvable key"

// Await blocks until the given key has been produced by another [Preemptable] function.
// It returns immediately if the key was already completed.
// This function must only be called from within [Run], using the given context.
func Await(ctx context.Context, key string) error {
	s, err := awaitStateFromCtx(ctx)
	if err != nil {
		return err
	}
	if _, done := s.completed[key]; done {
		return nil
	}

	resumeSignal := make(chan error)

	// Add self to waiting list for key.
	w := s.waiting[key]
	w = append(w, resumeSignal)
	s.waiting[key] = w

	// Signal that we are waiting.
	s.awaitc <- struct{}{}

	// We don't wait on ctx.IsDone(). This is checked [Run], which will still unblock us.
	return <-resumeSignal
}

// Preemptable is a function that can be scheduled by [Run].
// It returns one or more keys that other functions may be awaiting via [Await].
type Preemptable func(ctx context.Context) (keys []string, err error)

// Run executes the given functions cooperatively, one at a time. Functions may
// call [Await] to yield until a key produced by another function becomes available.
func Run(ctx context.Context, fns iter.Seq[Preemptable]) error {
	s := &awaitState{
		waiting:   make(map[string][]chan error),
		completed: make(map[string]struct{}),
		donec:     make(chan []string),
		awaitc:    make(chan struct{}),
	}

	ctx = context.WithValue(ctx, awaitStateKey{}, s)
	errs := errors.L()

	// Waits for the currently running goroutine to either
	// complete (donec)
	// yield via Await (awaitc), or for the context to be
	// cancelled. Returns false if the context was cancelled.
	waitForNextMessage := func() bool {
		select {
		case keys := <-s.donec:
			for _, key := range keys {
				s.completed[key] = struct{}{}
				waitingForKey, found := s.waiting[key]
				if found {
					s.resumable = append(s.resumable, waitingForKey...)
				}
				delete(s.waiting, key)
			}
			return true
		case <-s.awaitc:
			return true
		case <-ctx.Done():
			// Drain the in-flight goroutine.
			select {
			case <-s.donec:
			case <-s.awaitc:
			}
			return false
		}
	}

	cancelled := false
	for nextFn := range fns {
		go func() {
			key, err := nextFn(ctx)
			errs.Append(err)
			s.donec <- key
		}()

		if !waitForNextMessage() {
			cancelled = true
			break
		}

		for len(s.resumable) > 0 {
			nextResumable := s.resumable[0]
			s.resumable = s.resumable[1:]
			nextResumable <- nil
			if !waitForNextMessage() {
				cancelled = true
				break
			}
		}
		if cancelled {
			break
		}
	}

	// Unblock all waiting goroutines.
	for _, waitingForKey := range s.waiting {
		for _, resumeSignal := range waitingForKey {
			resumeSignal <- errors.E(ErrUnresolvable)
			<-s.donec
		}
	}

	if cancelled {
		errs.Append(ctx.Err())
	}

	return errs.AsError()
}

type awaitStateKey struct{}

type awaitState struct {
	waiting   map[string][]chan error
	completed map[string]struct{}
	resumable []chan error
	donec     chan []string
	awaitc    chan struct{}
}

func awaitStateFromCtx(ctx context.Context) (*awaitState, error) {
	if v := ctx.Value(awaitStateKey{}); v != nil {
		if u, ok := v.(*awaitState); ok {
			return u, nil
		}
		return nil, errors.E("invalid awaitState type")
	}
	return nil, errors.E("context contains no awaitState")
}
