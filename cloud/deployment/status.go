// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package deployment provides types and helpers for cloud deployments.
package deployment

import (
	"bytes"
	"encoding/json"
	"math/bits"
	"strconv"

	"github.com/terramate-io/terramate/errors"
)

const (
	// ErrInvalidStatus represents an invalid deployment status error.
	ErrInvalidStatus errors.Kind = "invalid deployment status"

	// ErrInvalidFilterStatus represents an invalid deployment status filter error.
	ErrInvalidFilterStatus errors.Kind = "invalid filter deployment status"
)

type (
	// Status of a deployment.
	Status uint8

	// FilterStatus represents a filter for deployment statuses.
	FilterStatus Status
)

const (
	OK           Status = 1 << iota // OK status is used when the stack ran successfully.
	Pending                         // Pending is a temporary status set when the deployment is about to commence.
	Running                         // Running is a temporary status as part of an still ongoing deployment.
	Failed                          // Failed status indicates the deployment of the stack failed.
	Canceled                        // Canceled indicates the deployment of the stack was canceled.
	Unrecognized                    // Unrecognized indicates any deployment status returned from TMC but not recognized by this client version.
	lastStatus
)

const (
	// UnhealthyFilter status is used for filtering not OK deployment status.
	UnhealthyFilter FilterStatus = FilterStatus(^OK) // assumes only 1 status is healthy

	// HealthyFilter status is used for filtering healthy statuses. Just [OK] for now.
	HealthyFilter FilterStatus = FilterStatus(OK)

	// AllFilter filters for any stacks statuses.
	AllFilter FilterStatus = HealthyFilter | UnhealthyFilter
	// NoFilter disables the filtering for statuses.
	NoFilter FilterStatus = 0
)

// NewStatus creates a new stack status from a string.
func NewStatus(str string) Status {
	var s Status
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
		*s = Unrecognized
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
	default:
		return "unrecognized (" + strconv.Itoa(int(s)) + ")"
	}
}

// Is tells if status matches the provided filter.
func (s Status) Is(filter FilterStatus) bool {
	return FilterStatus(s)&filter != 0
}

// NewStatusFilter creates a new filter for deployment statuses.
func NewStatusFilter(str string) (FilterStatus, error) {
	if str == "unhealthy" {
		return UnhealthyFilter, nil
	} else if str == "healthy" {
		return HealthyFilter, nil
	}

	s := NewStatus(str)
	if s == Unrecognized {
		return FilterStatus(0), errors.E("unrecognized deployment status filter: %s", str)
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

	for i := OK; i <= Canceled; i *= 2 {
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
