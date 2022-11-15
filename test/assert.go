// Copyright 2022 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package test

import "testing"

// AssertEqualSets checks if two sets contains the same elements
// independent of order (handles slices as sets).
func AssertEqualSets[T comparable](t *testing.T, got, want []T) {
	if len(got) != len(want) {
		t.Fatalf("got: %+v; want: %+v", got, want)
	}

loop:
	for _, w := range want {
		for j, g := range got {
			if g == w {
				got = append(got[:j], got[j+1:]...)
				continue loop
			}
		}
	}

	if len(got) > 0 {
		t.Fatalf("unable to find %v from got inside wanted set %v", got, want)
	}
}
