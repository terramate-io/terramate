// Copyright 2022 Mineiros GmbH
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

package event_test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/event"
)

func TestEventStream(t *testing.T) {
	stream := event.NewStream[int](3)

	assert.IsTrue(t, stream.Send(1))
	assert.IsTrue(t, stream.Send(2))
	assert.IsTrue(t, stream.Send(3))
	assert.IsTrue(t, !stream.Send(4))

	stream.Close()

	want := 1
	for event := range stream {
		assert.EqualInts(t, event, want)
		want++
	}
}
