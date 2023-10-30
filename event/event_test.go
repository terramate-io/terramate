// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package event_test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/event"
)

func TestEventStream(t *testing.T) {
	t.Parallel()
	stream := event.NewStream[int](3)

	assert.IsTrue(t, stream.Send(1))
	assert.IsTrue(t, stream.Send(2))
	assert.IsTrue(t, stream.Send(3))
	assert.IsTrue(t, !stream.Send(4))

	close(stream)

	want := 1
	for event := range stream {
		assert.EqualInts(t, event, want)
		want++
	}
}

func TestEventStreamZeroValueWontBlock(t *testing.T) {
	t.Parallel()
	var stream event.Stream[string]

	assert.IsTrue(t, !stream.Send("ok"))
	assert.IsTrue(t, !stream.Send("ok2"))
}
