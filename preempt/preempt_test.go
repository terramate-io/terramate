// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package preempt

import (
	"context"
	"fmt"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunEmptyFns(t *testing.T) {
	t.Parallel()
	err := Run(t.Context(), slices.Values([]Preemptable(nil)))
	if err != nil {
		t.Fatalf("expected nil error for empty fns, got: %v", err)
	}
}

func TestRunSingleFnNoAwait(t *testing.T) {
	t.Parallel()
	var called atomic.Bool
	fns := []Preemptable{
		func(_ context.Context) ([]string, error) {
			called.Store(true)
			return []string{"a"}, nil
		},
	}
	err := Run(t.Context(), slices.Values(fns))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called.Load() {
		t.Fatal("function was not called")
	}
}

func TestRunMultipleFnsNoAwait(t *testing.T) {
	t.Parallel()
	var count atomic.Int32
	fns := []Preemptable{
		func(_ context.Context) ([]string, error) {
			count.Add(1)
			return []string{"a"}, nil
		},
		func(_ context.Context) ([]string, error) {
			count.Add(1)
			return []string{"b"}, nil
		},
		func(_ context.Context) ([]string, error) {
			count.Add(1)
			return []string{"c"}, nil
		},
	}
	err := Run(t.Context(), slices.Values(fns))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := count.Load(); got != 3 {
		t.Fatalf("expected 3 calls, got %d", got)
	}
}

func TestRunFnReturnsError(t *testing.T) {
	t.Parallel()
	fns := []Preemptable{
		func(_ context.Context) ([]string, error) {
			return []string{"a"}, fmt.Errorf("something failed")
		},
	}
	err := Run(t.Context(), slices.Values(fns))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRunMultipleFnsWithErrors(t *testing.T) {
	t.Parallel()
	fns := []Preemptable{
		func(_ context.Context) ([]string, error) {
			return []string{"a"}, fmt.Errorf("error 1")
		},
		func(_ context.Context) ([]string, error) {
			return []string{"b"}, fmt.Errorf("error 2")
		},
	}
	err := Run(t.Context(), slices.Values(fns))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRunProducerBeforeConsumer(t *testing.T) {
	t.Parallel()
	// When the producer completes a key before the consumer starts awaiting it,
	// the consumer should still resolve via the completed set.
	var consumerRan atomic.Bool
	fns := []Preemptable{
		func(_ context.Context) ([]string, error) {
			return []string{"x"}, nil
		},
		func(ctx context.Context) ([]string, error) {
			err := Await(ctx, "x")
			if err != nil {
				return nil, err
			}
			consumerRan.Store(true)
			return []string{"done"}, nil
		},
	}
	err := Run(t.Context(), slices.Values(fns))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !consumerRan.Load() {
		t.Fatal("consumer did not run after awaiting already-completed key")
	}
}

func TestRunAwaitBeforeProducer(t *testing.T) {
	t.Parallel()
	// Consumer is scheduled first and awaits key "data".
	// Producer is scheduled second and completes key "data".
	// The consumer should be resumed after the producer finishes.
	fns := []Preemptable{
		func(ctx context.Context) ([]string, error) {
			err := Await(ctx, "data")
			if err != nil {
				return nil, err
			}
			return []string{"consumer-done"}, nil
		},
		func(_ context.Context) ([]string, error) {
			return []string{"data"}, nil
		},
	}
	err := Run(t.Context(), slices.Values(fns))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunMultipleAwaitersOnSameKey(t *testing.T) {
	t.Parallel()
	var resumed atomic.Int32
	fns := []Preemptable{
		func(ctx context.Context) ([]string, error) {
			err := Await(ctx, "shared")
			if err != nil {
				return nil, err
			}
			resumed.Add(1)
			return []string{"waiter1-done"}, nil
		},
		func(ctx context.Context) ([]string, error) {
			err := Await(ctx, "shared")
			if err != nil {
				return nil, err
			}
			resumed.Add(1)
			return []string{"waiter2-done"}, nil
		},
		func(_ context.Context) ([]string, error) {
			return []string{"shared"}, nil
		},
	}
	err := Run(t.Context(), slices.Values(fns))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := resumed.Load(); got != 2 {
		t.Fatalf("expected 2 resumed awaiters, got %d", got)
	}
}

func TestRunStuckDetection(t *testing.T) {
	t.Parallel()
	// Single function that awaits a key that nobody will ever produce.
	// The scheduler should detect the stuck state.
	fns := []Preemptable{
		func(ctx context.Context) ([]string, error) {
			err := Await(ctx, "never-produced")
			if err != nil {
				return nil, err
			}
			return []string{"unreachable"}, nil
		},
	}
	err := Run(t.Context(), slices.Values(fns))
	if err == nil {
		t.Fatal("expected stuck error, got nil")
	}
}

func TestRunChainedAwait(t *testing.T) {
	t.Parallel()
	// A -> awaits "step1" -> produces "step2"
	// B -> awaits "step2" -> produces "step3"
	// C -> produces "step1"
	// Expected: C runs, A resumes and produces step2, B resumes.
	var order []string
	fns := []Preemptable{
		func(ctx context.Context) ([]string, error) {
			if err := Await(ctx, "step1"); err != nil {
				return nil, err
			}
			order = append(order, "A-resumed")
			return []string{"step2"}, nil
		},
		func(ctx context.Context) ([]string, error) {
			if err := Await(ctx, "step2"); err != nil {
				return nil, err
			}
			order = append(order, "B-resumed")
			return []string{"step3"}, nil
		},
		func(_ context.Context) ([]string, error) {
			order = append(order, "C-done")
			return []string{"step1"}, nil
		},
	}
	err := Run(t.Context(), slices.Values(fns))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 3 {
		t.Fatalf("expected 3 events, got %d: %v", len(order), order)
	}
}

func TestRunMixedSuccessAndError(t *testing.T) {
	t.Parallel()
	fns := []Preemptable{
		func(_ context.Context) ([]string, error) {
			return []string{"a"}, nil
		},
		func(_ context.Context) ([]string, error) {
			return []string{"b"}, fmt.Errorf("b failed")
		},
		func(_ context.Context) ([]string, error) {
			return []string{"c"}, nil
		},
	}
	err := Run(t.Context(), slices.Values(fns))
	if err == nil {
		t.Fatal("expected error from fn that failed, got nil")
	}
}

func TestRunContextPassedToFns(t *testing.T) {
	t.Parallel()
	type ctxKey struct{}
	parentCtx := context.WithValue(t.Context(), ctxKey{}, "hello")

	var got atomic.Value
	fns := []Preemptable{
		func(ctx context.Context) ([]string, error) {
			if v, ok := ctx.Value(ctxKey{}).(string); ok {
				got.Store(v)
			}
			return []string{"a"}, nil
		},
	}
	err := Run(parentCtx, slices.Values(fns))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Load() != "hello" {
		t.Fatalf("expected parent context value 'hello', got %v", got.Load())
	}
}

func TestAwaitWithoutRunContextFails(t *testing.T) {
	t.Parallel()
	err := Await(t.Context(), "anything")
	if err == nil {
		t.Fatal("expected error calling Await outside of Run context")
	}
}

func TestRunConsumerThenProducerSameKey(t *testing.T) {
	t.Parallel()
	// When the consumer (awaiter) is scheduled before the producer,
	// it correctly blocks until the producer completes the key.
	var consumerRan atomic.Bool
	fns := []Preemptable{
		func(ctx context.Context) ([]string, error) {
			if err := Await(ctx, "x"); err != nil {
				return nil, err
			}
			consumerRan.Store(true)
			return []string{"y"}, nil
		},
		func(_ context.Context) ([]string, error) {
			return []string{"x"}, nil
		},
	}
	err := Run(t.Context(), slices.Values(fns))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !consumerRan.Load() {
		t.Fatal("consumer did not run after awaiting key")
	}
}

func TestRunCancelledContext(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())

	fns := []Preemptable{
		func(_ context.Context) ([]string, error) {
			cancel()
			return []string{"a"}, nil
		},
		func(ctx context.Context) ([]string, error) {
			// If the context is cancelled, Await should return ctx.Err()
			err := Await(ctx, "never")
			return nil, err
		},
	}
	err := Run(ctx, slices.Values(fns))
	// Either a stuck error or a context.Canceled error is acceptable
	if err == nil {
		t.Fatal("expected error after context cancellation, got nil")
	}
}

func TestRunLargeFanOut(t *testing.T) {
	t.Parallel()
	const n = 20
	var count atomic.Int32

	fns := make([]Preemptable, n)
	for i := range fns {
		key := fmt.Sprintf("key-%d", i)
		fns[i] = func(_ context.Context) ([]string, error) {
			count.Add(1)
			return []string{key}, nil
		}
	}

	done := make(chan error, 1)
	go func() {
		done <- Run(t.Context(), slices.Values(fns))
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not complete within timeout")
	}

	if got := count.Load(); got != n {
		t.Fatalf("expected %d calls, got %d", n, got)
	}
}

func TestRunMultipleKeysFromSingleFn(t *testing.T) {
	t.Parallel()
	// A single producer returns multiple keys at once.
	// Two consumers each await a different key from that producer.
	var aRan, bRan atomic.Bool
	fns := []Preemptable{
		func(ctx context.Context) ([]string, error) {
			if err := Await(ctx, "x"); err != nil {
				return nil, err
			}
			aRan.Store(true)
			return nil, nil
		},
		func(ctx context.Context) ([]string, error) {
			if err := Await(ctx, "y"); err != nil {
				return nil, err
			}
			bRan.Store(true)
			return nil, nil
		},
		func(_ context.Context) ([]string, error) {
			return []string{"x", "y"}, nil
		},
	}
	err := Run(t.Context(), slices.Values(fns))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !aRan.Load() {
		t.Fatal("consumer awaiting 'x' did not run")
	}
	if !bRan.Load() {
		t.Fatal("consumer awaiting 'y' did not run")
	}
}

func TestRunFnReturnsNoKeys(t *testing.T) {
	t.Parallel()
	var called atomic.Bool
	fns := []Preemptable{
		func(_ context.Context) ([]string, error) {
			called.Store(true)
			return nil, nil
		},
	}
	err := Run(t.Context(), slices.Values(fns))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called.Load() {
		t.Fatal("function was not called")
	}
}
