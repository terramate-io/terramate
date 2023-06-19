package cloud

import (
	"fmt"

	"github.com/terramate-io/terramate/errors"
)

// Status of a stack or deployment
type Status int

const (
	Unknown  Status = iota // Unknown status is used for newly created stacks, which never ran.
	Pending                // Pending status
	Running                // Running status indicates the stack is running.
	OK                     // OK status is used when the stack ran successfully.
	Drifted                // Drifted status is used when a stack definition is different from that of the current status.
	Failed                 // Failed status indicates the deployment of the stack failed.
	Canceled               // Canceled indicates the deployment of the stack was canceled.
	invalid
)

// Validate the status.
func (s Status) Validate() error {
	if s < 0 || s >= invalid {
		return errors.E(`invalid status`)
	}
	return nil
}

// String representation of the status.
func (s Status) String() string {
	switch s {
	case Unknown:
		return "unknown"
	case Pending:
		return "pending"
	case Running:
		return "ok"
	case Drifted:
		return "drifted"
	case Failed:
		return "failed"
	case Canceled:
		return "canceled"
	}
	panic(fmt.Sprintf("unrecognized status: %d", s))
}
