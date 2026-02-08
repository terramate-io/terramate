// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tui

import "testing"

func TestNormalizePluginCommand(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want string
	}{
		{in: "package create <output-dir>", want: "package create"},
		{in: "package create [<output-dir>]", want: "package create"},
		{in: "component create", want: "component create"},
		{in: "scaffold", want: "scaffold"},
	}
	for _, tc := range tests {
		if got := normalizePluginCommand(tc.in); got != tc.want {
			t.Fatalf("normalizePluginCommand(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
