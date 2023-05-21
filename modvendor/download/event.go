// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package download

import (
	"github.com/terramate-io/terramate/event"
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
