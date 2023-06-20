// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud

import (
	"encoding/json"
	"fmt"
	"strconv"

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

// MarshalJSON implements the Marshaller interface.
func (s Status) MarshalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return []byte(strconv.Quote(s.String())), nil
}

// UnmarshalJSON implements the Unmarshaller interface.
func (s *Status) UnmarshalJSON(b []byte) error {
	var str string
	if err := json.Unmarshal(b, &str); err != nil {
		return err
	}
	switch str {
	case "unknown":
		*s = Unknown
	case "ok":
		*s = OK
	case "pending":
		*s = Pending
	case "running":
		*s = Running
	case "drifted":
		*s = Drifted
	case "failed":
		*s = Failed
	case "canceled":
		*s = Canceled
	default:
		return errors.E("invalid status: %s", str)
	}
	return nil
}

// String representation of the status.
func (s Status) String() string {
	switch s {
	case Unknown:
		return "unknown"
	case OK:
		return "ok"
	case Pending:
		return "pending"
	case Running:
		return "running"
	case Drifted:
		return "drifted"
	case Failed:
		return "failed"
	case Canceled:
		return "canceled"
	}
	panic(fmt.Sprintf("unrecognized status: %d", s))
}
