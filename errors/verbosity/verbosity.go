// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package verbosity defines the common Terramate error verbosity levels.
package verbosity

const (
	// V0 is no verbosity.
	V0 int = iota
	// V1 is default verbosity level.
	V1
	// V2 is high verbosity level.
	V2
)
