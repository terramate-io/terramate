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

// Package event implements a simple event stream.
package event

// Stream is a stream of events.
type Stream[T any] chan T

// NewStream creates a new stream.
func NewStream[T any](buffsize int) Stream[T] {
	return Stream[T](make(chan T, buffsize))
}

// Send event on this event stream. Returns true if stream is not full,
// false if the stream is full.
func (s Stream[T]) Send(event T) bool {
	select {
	case s <- event:
		return true
	default:
		return false
	}
}

// Close the event  this event stream. Should not be called more than
// once and after closing Send should not be called.
func (s Stream[T]) Close() {
	close(s)
}
