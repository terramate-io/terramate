// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package drift provides types and helpers for cloud drifts.
package drift

import (
	"bytes"
	"encoding/json"
	"math/bits"
	"strconv"

	"github.com/terramate-io/terramate/errors"
)

const (
	// ErrInvalidStatus represents an invalid drift status error.
	ErrInvalidStatus errors.Kind = "invalid drift status"
	// ErrInvalidFilterStatus represents an invalid drift status filter error.
	ErrInvalidFilterStatus errors.Kind = "invalid drift stack filter"
)

type (
	// Status of a drift.
	Status uint8

	// FilterStatus represents a filter for drift statuses.
	FilterStatus Status
)

const (
	// OK status is used when the stack is not drifted.
	OK Status = 1 << iota
	// Unknown indicates the drift detection was not executed yet.
	Unknown
	// Drifted status indicates the stack is drifted.
	Drifted
	// Failed status indicates the drift detection of the stack failed.
	Failed
	// Unrecognized indicates any drift status returned from TMC but not
	// recognized by this client version.
	Unrecognized
	lastStatus = Failed
)

const (
	// UnhealthyFilter status is used for filtering not OK drift status.
	UnhealthyFilter FilterStatus = FilterStatus(^OK)
	// HealthyFilter status is used for filtering for healthy statuses. Just [OK] for now.
	HealthyFilter FilterStatus = FilterStatus(OK)
	// NoFilter disables the filtering for statuses.
	NoFilter FilterStatus = 0
)

// NewStatus creates a new stack drift status from a string.
func NewStatus(str string) Status {
	var s Status
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
		*s = Unrecognized
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
	default:
		return "unrecognized (" + strconv.Itoa(int(s)) + ")"
	}
}

// Is tells if status matches the provided filter.
func (s Status) Is(filter FilterStatus) bool {
	return FilterStatus(s)&filter != 0
}

// NewStatusFilter creates a new filter for drift statuses.
func NewStatusFilter(str string) (FilterStatus, error) {
	if str == "unhealthy" {
		return UnhealthyFilter, nil
	} else if str == "healthy" {
		return HealthyFilter, nil
	}

	s := NewStatus(str)
	if s == Unrecognized {
		return FilterStatus(0), errors.E("unrecognized drift status filter: %s", str)
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

	for i := OK; i <= Failed; i *= 2 {
		s := Status(i)
		if Status(f)&s > 0 {
			if out.Len() > 0 {
				_, _ = out.WriteRune('|')
			}
			_, _ = out.WriteString(s.String())
		}
	}
	return out.String()
}
