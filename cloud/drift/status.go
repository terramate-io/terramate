// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package drift provides types and helpers for cloud drifts.
package drift

import (
	"encoding/json"
	"fmt"
	"math/bits"
	"strconv"

	"github.com/terramate-io/terramate/errors"
)

const (
	// ErrInvalidStatus represents an invalid drift status error.
	ErrInvalidStatus errors.Kind = "invalid drift status"
)

// Status of a drift.
type Status uint8

const (
	OK         Status = 1 << iota // OK status is used when the stack is not drifted.
	Unknown                       // Unknown indicates the drift detection was not executed yet.
	Drifted                       // Drifted status indicates the stack is drifted.
	Failed                        // Failed status indicates the drift detection of the stack failed.
	lastStatus = Failed
)

// NewStatus creates a new stack drift status from a string.
func NewStatus(str string) (Status, error) {
	var s Status
	err := s.UnmarshalJSON([]byte(strconv.Quote(str)))
	return s, err
}

// Validate the status.
func (s Status) Validate() error {
	// each status has only 1 bit set.
	if nbits := bits.OnesCount8(uint8(s)); nbits != 1 || s > lastStatus {
		return errors.E(ErrInvalidStatus)
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

// UnmarshalJSON unmarshals drift status from JSONs.
func (s *Status) UnmarshalJSON(b []byte) error {
	var str string
	if err := json.Unmarshal(b, &str); err != nil {
		return err
	}
	switch str {
	case "ok":
		*s = OK
	case "unknown":
		*s = Unknown
	case "drifted":
		*s = Drifted
	case "failed":
		*s = Failed
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
	case Unknown:
		return "unknown"
	case Drifted:
		return "drifted"
	case Failed:
		return "failed"
	}
	panic(fmt.Sprintf("unrecognized status: %d", s))
}
