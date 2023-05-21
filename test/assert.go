// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package test

import "testing"

// AssertEqualSets checks if two sets contains the same elements
// independent of order (handles slices as sets).
func AssertEqualSets[T comparable](t *testing.T, got, want []T) {
	if len(got) != len(want) {
		t.Fatalf("got: %+v; want: %+v", got, want)
	}

	for _, w := range want {
		for i, g := range got {
			if g == w {
				got = append(got[:i], got[i+1:]...)
				break
			}
		}
	}

	if len(got) > 0 {
		t.Fatalf("unable to find %v from got inside wanted set %v", got, want)
	}
}
