// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package stack provides types and helpers for cloud stacks.
package stack

import (
	"bytes"
	"encoding/json"
	"math/bits"
	"strconv"

	"github.com/terramate-io/terramate/errors"
)

const (
	// ErrInvalidStatus represents an invalid stack status error.
	ErrInvalidStatus errors.Kind = "invalid stack status"

	// ErrInvalidFilterStatus represents an invalid filter stack status error.
	ErrInvalidFilterStatus errors.Kind = "invalid filter stack status"
)

type (
	// Status of a stack.
	Status uint8

	// FilterStatus represents a filter for stack statuses.
	FilterStatus Status
)

const (
	OK           Status = 1 << iota // OK status is used when the stack ran successfully.
	Unknown                         // Unknown status is used for newly created stacks, which never ran.
	Drifted                         // Drifted status is used when a stack definition is different from that of the current status.
	Failed                          // Failed status indicates the deployment of the stack failed.
	Canceled                        // Canceled indicates the deployment of the stack was canceled.
	Unrecognized                    // Unrecognized indicates any status returned from TMC but still not recognized by the client.
	lastStatus   = Unrecognized
)

const (
	// UnhealthyFilter status is used for filtering not Ok status.
	UnhealthyFilter FilterStatus = FilterStatus(Unknown | Drifted | Failed | Canceled | Unrecognized)
	// AllFilter filters for any stacks statuses.
	AllFilter FilterStatus = FilterStatus(OK) | UnhealthyFilter
	// NoFilter disables the filtering for statuses.
	NoFilter FilterStatus = 0
)

// NewStatus creates a new stack status from a string.
func NewStatus(str string) Status {
	var s Status
	// it should work for any quoted string.
	_ = s.UnmarshalJSON([]byte(strconv.Quote(str)))
	return s
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

// UnmarshalJSON unmarshals stack status from JSONs.
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
	case "canceled":
		*s = Canceled
	default:
		*s = Unrecognized
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
	case Drifted:
		return "drifted"
	case Failed:
		return "failed"
	case Canceled:
		return "canceled"
	default:
		return "unrecognized"
	}
}

// NewStatusFilter creates a new filter for stack statuses.
func NewStatusFilter(str string) FilterStatus {
	if str == "unhealthy" {
		return UnhealthyFilter
	}

	return FilterStatus(NewStatus(str))
}

// Validate the status filter.
func (f FilterStatus) Validate() error {
	if f > UnhealthyFilter {
		return errors.E(ErrInvalidFilterStatus)
	}
	return nil
}

func (f FilterStatus) String() string {
	if f == UnhealthyFilter {
		return "unhealthy"
	}
	var out bytes.Buffer

	for _, s := range []Status{OK, Drifted, Canceled, Unknown} {
		if Status(f)&s > 0 {
			if out.Len() > 0 {
				_, _ = out.WriteRune('|')
			}
			_, _ = out.WriteString(s.String())
		}
	}
	return out.String()
}
