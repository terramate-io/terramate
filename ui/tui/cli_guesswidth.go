// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build !unix

package tui

import "io"

func guessWidth(_ io.Writer) int {
	return 80
}
