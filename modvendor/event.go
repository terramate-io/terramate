package modvendor

import "github.com/mineiros-io/terramate/event"

// ProgressEvent represents a vendor progress event.
type ProgressEvent struct {
}

// EventStream is a stream of vendor related events.
type EventStream event.Stream[ProgressEvent]

// NewEventStream creates a new event stream.
func NewEventStream() EventStream {
	return EventStream(event.NewStream[ProgressEvent](100))
}
