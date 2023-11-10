// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stack

import (
	"github.com/terramate-io/terramate/errors"
	"golang.org/x/exp/slices"
)

const (
	// ErrInvalidFilterStatus represents an invalid filter stack status error.
	ErrInvalidFilterStatus errors.Kind = "invalid filter stack status"
)

type (
	// FilterStatus represents the query paramater used for filtering stacks via the API
	FilterStatus string
)

const (
	FilterStatusDrifted   FilterStatus = "drifted"   // FilterStatusDrifted represents drifted status filter
	FilterStatusOK        FilterStatus = "ok"        // FilterStatusOK represents an ok status filter
	FilterStatusFailed    FilterStatus = "failed"    // FilterStatusFailed represents a failed status filter
	FilterStatusHealthy   FilterStatus = "healthy"   // FilterStatusHealthy represents a healthy status filter
	FilterStatusUnhealthy FilterStatus = "unhealthy" // FilterStatusUnhealthy represents an unhealthy status filter
)

const (
	NoFilter                 FilterStatus = ""             // NoFilter represents an empty status filter
	FilterStatusUnrecognized FilterStatus = "unrecognized" // FilterStatusUnrecognized represents an unrecognized/unsupported status filter
)

// NewFilterStatus parses the provided status into a Status
func NewFilterStatus(status string) FilterStatus {
	switch status {
	case "":
		return NoFilter
	case "healthy":
		return FilterStatusHealthy
	case "unhealthy":
		return FilterStatusUnhealthy
	case "ok":
		return FilterStatusOK
	case "drifted":
		return FilterStatusDrifted
	case "failed":
		return FilterStatusFailed
	default:
		return FilterStatusUnrecognized
	}
}

func (s FilterStatus) String() string {
	return string(s)
}

// MetaEquals allows callers to compare `s` with the provided `status` e.g.
// "s.MetaEquals("ok") returns true when s == StatusHealthy
// "s.MetaEquals("drifted") returns true when s == StatusUnhealthy
// "s.MetaEquals("failed") returns true when s == StatusUnhealthy
// "s.MetaEquals("ok") returns true when s == StatusOK
// "s.MetaEquals("ok") returns false when s == StatusOK
// and so on..
func (s FilterStatus) MetaEquals(status string) bool {
	parsedStatus := NewFilterStatus(status)
	switch s {
	case FilterStatusHealthy:
		return isHealthyStatus(parsedStatus)
	case FilterStatusUnhealthy:
		return isUnhealthyStatus(parsedStatus)
	default:
		return parsedStatus == s
	}
}

func isHealthyStatus(status FilterStatus) bool {
	return slices.Contains(healthyStatuses(), status)
}

func isUnhealthyStatus(status FilterStatus) bool {
	return slices.Contains(unhealthyStatuses(), status)
}

func healthyStatuses() []FilterStatus {
	return []FilterStatus{FilterStatusOK, FilterStatusHealthy}
}
func unhealthyStatuses() []FilterStatus {
	return []FilterStatus{FilterStatusDrifted, FilterStatusFailed, FilterStatusUnhealthy}
}
