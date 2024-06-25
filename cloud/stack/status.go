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
	Drifted                         // Drifted status is used when a stack definition is different from that of the current status.
	Failed                          // Failed status indicates the deployment of the stack failed.
	Unrecognized                    // Unrecognized indicates any status returned from TMC but still not recognized by the client.
	lastStatus   = Unrecognized
)

const (
	// UnhealthyFilter status is used for filtering not Ok status.
	UnhealthyFilter FilterStatus = FilterStatus(Drifted | Failed)

	// HealthyFilter status is used for filtering healthy statuses. Just [OK] for now.
	HealthyFilter FilterStatus = FilterStatus(OK)

	// AllFilter filters for any stacks statuses.
	AllFilter FilterStatus = HealthyFilter | UnhealthyFilter
	// NoFilter disables the filtering for statuses.
	NoFilter FilterStatus = 0
)

// AnyTarget is the empty deployment target filter that includes any target.
const AnyTarget = ""

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
		return errors.E(ErrInvalidStatus, "%s", s)
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
	case "drifted":
		*s = Drifted
	case "failed":
		*s = Failed
	default:
		*s = Unrecognized
	}
	return nil
}

// String representation of the status.
func (s Status) String() string {
	switch s {
	case OK:
		return "ok"
	case Drifted:
		return "drifted"
	case Failed:
		return "failed"
	default:
		return "unrecognized (" + strconv.Itoa(int(s)) + ")"
	}
}

// Is tells if status matches the provided filter.
func (s Status) Is(filter FilterStatus) bool {
	return FilterStatus(s)&filter != 0
}

// NewStatusFilter creates a new filter for stack statuses.
func NewStatusFilter(str string) (FilterStatus, error) {
	if str == "unhealthy" {
		return UnhealthyFilter, nil
	} else if str == "healthy" {
		return HealthyFilter, nil
	}
	s := NewStatus(str)
	if s == Unrecognized {
		return FilterStatus(0), errors.E("unrecognized status filter: %s", str)
	}
	return FilterStatus(s), nil
}

// Is tells if the filter matches the provided status.
func (f FilterStatus) Is(status Status) bool {
	return status.Is(f)
}

func (f FilterStatus) String() string {
	if f == UnhealthyFilter {
		return "unhealthy"
	}
	var out bytes.Buffer

	for _, s := range []Status{OK, Drifted, Failed} {
		if Status(f)&s > 0 {
			if out.Len() > 0 {
				_, _ = out.WriteRune('|')
			}
			_, _ = out.WriteString(s.String())
		}
	}
	return out.String()
}
