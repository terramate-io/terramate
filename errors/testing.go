package errors

import "testing"

// Assert that got is of same error kind as want.
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
		t.Fatal("got is not a *errors.Error")
	}

	e2, ok := want.(*Error)
	if !ok {
		t.Fatal("want is not a *errors.Error")
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
