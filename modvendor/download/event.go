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

package download

import (
	"github.com/mineiros-io/terramate/event"
)

// ProgressEventStream is a stream of vendor related events.
type ProgressEventStream event.Stream[event.VendorProgress]

// NewEventStream creates a new event stream.
func NewEventStream() ProgressEventStream {
	const streamBufferSize = 100
	return ProgressEventStream(event.NewStream[event.VendorProgress](streamBufferSize))
}

// Send send a progress event.
func (e ProgressEventStream) Send(pe event.VendorProgress) bool {
	return event.Stream[event.VendorProgress](e).Send(pe)
}
