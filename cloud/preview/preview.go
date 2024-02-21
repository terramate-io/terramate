// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package preview

import "github.com/terramate-io/terramate/errors"

// StackStatus is the status of a stack in a preview run
type StackStatus string

const (
	// StackStatusAffected is the status for a stack that is affected in a PR
	StackStatusAffected StackStatus = "affected"
	// StackStatusPending is the status for a stack that is selected in a preview run
	StackStatusPending StackStatus = "pending"
	// StackStatusRunning is the status for a stack that is currently running in a preview run
	StackStatusRunning StackStatus = "running"
	// StackStatusUnchanged is the status for a stack that has no changes in a preview run (successful exit code 0)
	StackStatusUnchanged StackStatus = "unchanged"
	// StackStatusChanged is the status for a stack that has changes in a preview run (successful exit code 2)
	StackStatusChanged StackStatus = "changed"
	// StackStatusCanceled is the status for a stack that has been canceled in a preview run
	StackStatusCanceled StackStatus = "canceled"
	// StackStatusFailed is the status for a stack that has failed in a preview run (non-successful exit code)
	StackStatusFailed StackStatus = "failed"
)

// ErrInvalidStackStatus represents an invalid stack status
const ErrInvalidStackStatus = errors.Kind("invalid stack status")

func (p StackStatus) String() string {
	return string(p)
}

// Validate validates the stack status
func (p StackStatus) Validate() error {
	switch p {
	case StackStatusAffected,
		StackStatusPending,
		StackStatusRunning,
		StackStatusUnchanged,
		StackStatusChanged,
		StackStatusCanceled,
		StackStatusFailed:
		return nil
	default:
		return errors.E(ErrInvalidStackStatus, "unrecognized value")
	}
}

// DerivePreviewStatus derives the preview status from the exit code of
// a preview command
func DerivePreviewStatus(exitCode int) StackStatus {
	switch exitCode {
	case 2:
		return StackStatusChanged
	case 0:
		return StackStatusUnchanged
	case -1:
		return StackStatusCanceled
	default:
		return StackStatusFailed
	}
}
