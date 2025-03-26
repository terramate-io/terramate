// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package exit provides standard exit codes for Terramate.
package exit

// Status represents the exit status of a command.
type Status int

// Standard exit codes of Terramate
const (
	OK Status = iota
	Failed
	Changed

	// this can be extended by external commands.
)
