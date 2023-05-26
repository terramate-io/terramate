// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package errors provides useful assert functions for handling errors on tests
package errors

import (
	"fmt"
	"testing"

	"github.com/terramate-io/terramate/errors"
)

// AssertKind asserts that got is of same error kind as want.
func AssertKind(t *testing.T, got, want error) {
	t.Helper()
	if (got == nil) != (want == nil) {
		t.Fatalf("got error[%v] differs from want[%v]", got, want)
	}
	if want == nil {
		return
	}
	e1, ok := got.(*errors.Error)
	if !ok {
		t.Fatalf("got %v is not an *errors.Error", got)
	}

	e2, ok := want.(*errors.Error)
	if !ok {
		t.Fatalf("want %v is not an *errors.Error", want)
	}

	AssertIsKind(t, e1, e2.Kind)
}

// AssertIsKind asserts err is of kind k.
func AssertIsKind(t *testing.T, err error, k errors.Kind) {
	t.Helper()
	if !errors.IsKind(err, k) {
		t.Fatalf("error[%v] is not of kind %q", err, k)
	}
}

// Assert err is (contains, wraps, etc) target.
func Assert(t *testing.T, err, target error, args ...interface{}) {
	t.Helper()
	fmtctx := ""

	if len(args) > 0 {
		fmtctx = fmt.Sprintf(args[0].(string), args[1:]...)
	}

	if !errors.Is(err, target) {
		t.Fatalf("error[%s] is not target[%s]%s", errstr(err), errstr(target), fmtctx)
	}
}

// AssertIsErrors will check that all target errors are contained
// within the given err. Usually err underlying implementation is
// an errors.List, but that is not enforced, it is enough that for
// all the target errors errors.Is returns true, so this function also
// works for long chains of errors.
func AssertIsErrors(t *testing.T, err error, targets []error) {
	t.Helper()

	if err != nil && len(targets) == 0 {
		t.Fatalf("wanted no errors but got: %v", err)
	}

	for _, target := range targets {
		Assert(t, err, target)
	}
}

// AssertErrorList will check that the given err is an *errors.List
// and that all given errors on targets are contained on it
// using errors.Is.
func AssertErrorList(t *testing.T, err error, targets []error) {
	t.Helper()

	if err != nil {
		AssertAsErrorsList(t, err)
	}
	AssertIsErrors(t, err, targets)
}

// AssertAsErrorsList will check if the given error can be handled
// as an *errors.List by calling errors.As. It fails if the error fails
// to be an *errors.List.
func AssertAsErrorsList(t *testing.T, err error) {
	t.Helper()

	var errs *errors.List
	if !errors.As(err, &errs) {
		t.Fatalf("error %v doesn't match type %T", err, errs)
	}
}

func errstr(err error) string {
	if err == nil {
		return "<nil>"
	}
	if e, ok := err.(interface{ Detailed() string }); ok {
		return e.Detailed()
	}
	return err.Error()
}
