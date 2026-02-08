// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package plugin

import "testing"

func TestIsCompatible(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		baseVersion    string
		constraint     string
		wantCompatible bool
		wantErr        bool
	}{
		{"empty constraint", "1.0.0", "", true, false},
		{"compatible", "1.2.0", ">= 1.0.0, < 2.0.0", true, false},
		{"incompatible", "2.1.0", ">= 1.0.0, < 2.0.0", false, false},
		{"invalid base", "not-a-version", ">=1.0.0", false, true},
		{"invalid constraint", "1.0.0", "invalid", false, true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ok, err := IsCompatible(tt.baseVersion, tt.constraint)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ok != tt.wantCompatible && err == nil {
				t.Fatalf("expected %v got %v", tt.wantCompatible, ok)
			}
		})
	}
}
