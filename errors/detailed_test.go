// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package errors_test

import (
	stdfmt "fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/errors/verbosity"
)

func TestDetailedErrorInspect(t *testing.T) {
	t.Parallel()

	innerErr := errors.D("innerErr").
		WithDetailf(verbosity.V0, "inner.V0").
		WithDetailf(verbosity.V1, "inner.V1").
		WithDetailf(verbosity.V2, "inner.V2")

	outerErr := errors.D("outerErr").
		WithCause(innerErr).
		WithDetailf(verbosity.V0, "outer.V0").
		WithDetailf(verbosity.V1, "outer.V1").
		WithDetailf(verbosity.V2, "outer.V2")

	type entry struct {
		Index   int
		Msg     string
		Cause   error
		Details []errors.ErrorDetails
	}

	want := []entry{
		{
			Index: 0,
			Msg:   "outerErr",
			Cause: innerErr,
			Details: []errors.ErrorDetails{
				{Verbosity: 0, Msg: "outer.V0"},
				{Verbosity: 1, Msg: "outer.V1"},
				{Verbosity: 2, Msg: "outer.V2"},
			},
		},
		{
			Index: 1,
			Msg:   "innerErr",
			Cause: nil,
			Details: []errors.ErrorDetails{
				{Verbosity: 0, Msg: "inner.V0"},
				{Verbosity: 1, Msg: "inner.V1"},
				{Verbosity: 2, Msg: "inner.V2"},
			},
		},
	}

	got := []entry{}
	outerErr.Inspect(func(i int, msg string, cause error, details []errors.ErrorDetails) {
		got = append(got, entry{Index: i, Msg: msg, Cause: cause, Details: details})
	})

	if diff := cmp.Diff(got, want, cmpopts.EquateErrors()); diff != "" {
		t.Fatal(diff)
	}

	// Add a non-DetailedError on top
	otherErr := stdfmt.Errorf("otherErr")
	_ = innerErr.WithCause(otherErr)
	want[1].Cause = otherErr

	want = append(want, entry{
		Index: 2,
		Msg:   "otherErr",
	})

	got = []entry{}
	outerErr.Inspect(func(i int, msg string, cause error, details []errors.ErrorDetails) {
		got = append(got, entry{Index: i, Msg: msg, Cause: cause, Details: details})
	})

	if diff := cmp.Diff(got, want, cmpopts.EquateErrors()); diff != "" {
		t.Fatal(diff)
	}
}

func TestDetailedErrorCode(t *testing.T) {
	t.Parallel()

	codeInner := errors.Kind("err_code_1")
	codeOuter := errors.Kind("err_code_2")

	innerErr := errors.D("innerErr").
		WithCode(codeInner)

	outerErr := errors.D("outerErr").
		WithCode(codeOuter).
		WithCause(innerErr)

	assert.IsTrue(t, errors.HasCode(innerErr, codeInner), "innerErr has code_inner")
	assert.IsTrue(t, !errors.HasCode(innerErr, codeOuter), "innerErr has not code_outer")

	assert.IsTrue(t, errors.HasCode(outerErr, codeInner), "outerErr has code_inner")
	assert.IsTrue(t, errors.HasCode(outerErr, codeOuter), "outerErr has code_outer")
}
