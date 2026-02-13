// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package yaml

// Error is wrapper for an yaml error that stores a line and column numbers.
type Error struct {
	Err    error
	Line   int
	Column int
}

func (e Error) Error() string { return e.Err.Error() }

func (e Error) Unwrap() error { return e.Err }
