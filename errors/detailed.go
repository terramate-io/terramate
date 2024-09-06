// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package errors

import (
	"fmt"
)

// ErrorDetails contains a detailed message related to an error with a verbosity level.
type ErrorDetails struct {
	Msg       string
	Verbosity int
}

// DetailedError represents an error with additional detail information.
// These could be details about the error, hints, instructions etc.
// Optionally, the error might reference another error that caused it.
type DetailedError struct {
	Msg     string
	Code    Kind
	Cause   error
	Details []ErrorDetails
}

// Error implements the error interface to return a string representation.
// It only shows the main message and ignores additional details.
func (e DetailedError) Error() string {
	return e.Msg
}

// Is tells if e matches the target error.
// The target error must be of type DetailedError and it will try to match the following fields:
// - Msg
// - Details
// - Tags (match any)
// Any fields absent (empty) on the target error are ignored even if they exist on err (partial match).
func (e *DetailedError) Is(target error) bool {
	t, ok := target.(*DetailedError)
	if !ok {
		return false
	}
	if t.Msg != "" && e.Msg != t.Msg {
		return false
	}
	if t.Code != "" && e.Code != t.Code {
		return false
	}
	if t.Details != nil {
		if len(t.Details) != len(e.Details) {
			return false
		}
		for i, x := range e.Details {
			if x != t.Details[i] {
				return false
			}
		}
	}
	return true
}

// Unwrap returns the wrapped error, if there is any.
// Returns nil if there is no wrapped error.
func (e *DetailedError) Unwrap() error {
	return e.Cause
}

// WithCause sets the wrapped cause of the error.
// The caller is modified but also returned for convenience.
func (e *DetailedError) WithCause(cause error) *DetailedError {
	e.Cause = cause
	return e
}

// WithDetailf adds details to an error with the given verbosity level.
// The caller is modified but also returned for convenience.
func (e *DetailedError) WithDetailf(verbosity int, format string, a ...any) *DetailedError {
	e.Details = append(e.Details, ErrorDetails{Msg: fmt.Sprintf(format, a...), Verbosity: verbosity})
	return e
}

// WithDetail adds details to an error with the given verbosity level.
// The caller is modified but also returned for convenience.
func (e *DetailedError) WithDetail(verbosity int, msg string) *DetailedError {
	e.Details = append(e.Details, ErrorDetails{Msg: msg, Verbosity: verbosity})
	return e
}

// WithCode tags the error with an error code.
// This field is useful for error testing.
// The caller is modified but also returned for convenience.
func (e *DetailedError) WithCode(code Kind) *DetailedError {
	e.Code = code
	return e
}

// D is a constructor function to create a new DetailedError with a formatted message.
func D(format string, a ...any) *DetailedError {
	return &DetailedError{Msg: fmt.Sprintf(format, a...)}
}

// Inspect traverses a hierarchy of DetailedErrors along the cause field and invokes
// the given traversal function with the data from each item.
// If the type of the error is not DetailedError, it's still passed to the traversal function, but the traversal ends.
// If cause is nil, the traversal ends.
func (e DetailedError) Inspect(f func(i int, msg string, cause error, details []ErrorDetails)) {
	e.doInspect(0, f)
}

func (e DetailedError) doInspect(i int, f func(i int, msg string, cause error, details []ErrorDetails)) {
	f(i, e.Msg, e.Cause, e.Details)
	if e.Cause != nil {
		if c, ok := e.Cause.(*DetailedError); ok {
			c.doInspect(i+1, f)
		} else {
			f(i+1, e.Cause.Error(), nil, nil)
		}
	}
}

// HasCode tells if the error tree rooted at err contains the given code.
func HasCode(err error, code Kind) bool {
	return Is(err, &DetailedError{Code: code})
}
