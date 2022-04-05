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

package errors

import "testing"

// AssertKind asserts that got is of same error kind as want.
func AssertKind(t *testing.T, got, want error) {
	t.Helper()
	if (got == nil) != (want == nil) {
		t.Fatalf("got error[%v] differs from want[%v]", got, want)
	}
	if want == nil {
		return
	}
	e1, ok := got.(*Error)
	if !ok {
		t.Fatal("got is not an *errors.Error")
	}

	e2, ok := want.(*Error)
	if !ok {
		t.Fatal("want is not an *errors.Error")
	}

	AssertIsKind(t, e1, e2.Kind)
}

// AssertIsKind asserts err is of kind k.
func AssertIsKind(t *testing.T, err error, k Kind) {
	t.Helper()
	if !IsKind(err, k) {
		t.Fatalf("error[%v] is not of kind %q", err, k)
	}
}

// Assert err is (contains, wraps, etc) target.
func Assert(t *testing.T, err, target error) {
	t.Helper()
	if !Is(err, target) {
		t.Fatalf("error[%v] is not target[%v]", err, target)
	}
}
