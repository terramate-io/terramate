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

package modvendor

import (
	"github.com/mineiros-io/terramate/event"
	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/tf"
)

// ProgressEvent represents a vendor progress event.
type ProgressEvent struct {
	Message   string
	TargetDir project.Path
	Module    tf.Source
}

// EventStream is a stream of vendor related events.
type EventStream event.Stream[ProgressEvent]

// NewEventStream creates a new event stream.
func NewEventStream() EventStream {
	return EventStream(event.NewStream[ProgressEvent](100))
}

// Send send a progress event.
func (e EventStream) Send(pe ProgressEvent) bool {
	return event.Stream[ProgressEvent](e).Send(pe)
}
