// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package deployment provides types and helpers for cloud deployments.
package deployment

import (
	"encoding/json"
	"fmt"
	"math/bits"
	"strconv"

	"github.com/terramate-io/terramate/errors"
)

const (
	// ErrInvalidStatus represents an invalid deployment status error.
	ErrInvalidStatus errors.Kind = "invalid deployment status"
)

// Status of a stack or deployment
type Status uint8

const (
	OK       Status = 1 << iota // OK status is used when the stack ran successfully.
	Pending                     // Pending is a temporary status set when the deployment is about to commence.
	Running                     // Running is a temporary status as part of an still ongoing deployment.
	Failed                      // Failed status indicates the deployment of the stack failed.
	Canceled                    // Canceled indicates the deployment of the stack was canceled.

	lastStatus = Canceled
)

// NewStatus creates a new stack status from a string.
func NewStatus(str string) (Status, error) {
	var s Status
	err := s.UnmarshalJSON([]byte(strconv.Quote(str)))
	return s, err
}

// Validate the status.
func (s Status) Validate() error {
	// each status has only 1 bit set.
	if nbits := bits.OnesCount8(uint8(s)); nbits != 1 || s > lastStatus {
		return errors.E(ErrInvalidStatus, "%s", s)
	}
	return nil
}

// IsFinalState tells if the status is a final deployment state.
// The deployment state is a finite state machine where final states can be only
// OK, Failed or Canceled.
func (s Status) IsFinalState() bool {
	return s == OK || s == Failed || s == Canceled
}

// MarshalJSON implements the Marshaller interface.
func (s Status) MarshalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return []byte(strconv.Quote(s.String())), nil
}

// UnmarshalJSON unmarshals stack status from JSONs.
func (s *Status) UnmarshalJSON(b []byte) error {
	var str string
	if err := json.Unmarshal(b, &str); err != nil {
		return err
	}
	switch str {
	case "ok":
		*s = OK
	case "pending":
		*s = Pending
	case "running":
		*s = Running
	case "failed":
		*s = Failed
	case "canceled":
		*s = Canceled
	default:
		return errors.E(ErrInvalidStatus, str)
	}
	return nil
}

// String representation of the status.
func (s Status) String() string {
	switch s {
	case OK:
		return "ok"
	case Pending:
		return "pending"
	case Running:
		return "running"
	case Failed:
		return "failed"
	case Canceled:
		return "canceled"
	}
	panic(fmt.Sprintf("unrecognized status: %d", s))
}
