// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package preview_test

import (
	"errors"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cloud/preview"
)

func TestStackStatus(t *testing.T) {
	t.Parallel()
	type want struct {
		validationErr error
	}
	type testcase struct {
		name        string
		stackStatus string
		want        want
	}

	testcases := []testcase{
		{name: "affected", stackStatus: "affected", want: want{validationErr: nil}},
		{name: "pending", stackStatus: "pending", want: want{validationErr: nil}},
		{name: "running", stackStatus: "running", want: want{validationErr: nil}},
		{name: "unchanged", stackStatus: "unchanged", want: want{validationErr: nil}},
		{name: "changed", stackStatus: "changed", want: want{validationErr: nil}},
		{name: "canceled", stackStatus: "canceled", want: want{validationErr: nil}},
		{name: "failed", stackStatus: "failed", want: want{validationErr: nil}},
		{name: "empty", stackStatus: "", want: want{validationErr: errors.New("invalid stack status: unrecognized value")}},
		{name: "somestatus", stackStatus: "", want: want{validationErr: errors.New("invalid stack status: unrecognized value")}},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			status := preview.StackStatus(tc.stackStatus)
			err := status.Validate()
			assert.EqualErrs(t, tc.want.validationErr, err, "unexpected validation error")
		})
	}
}

func TestDerivePreviewStatus(t *testing.T) {
	t.Parallel()
	type want struct {
		status preview.StackStatus
	}
	type testcase struct {
		name     string
		exitCode int
		want     want
	}

	testcases := []testcase{
		{
			name:     "exit code -1",
			exitCode: -1,
			want: want{
				status: preview.StackStatusCanceled,
			},
		},
		{
			name:     "exit code 0",
			exitCode: 0,
			want: want{
				status: preview.StackStatusUnchanged,
			},
		},
		{
			name:     "exit code 1",
			exitCode: 1,
			want: want{
				status: preview.StackStatusFailed,
			},
		},
		{
			name:     "exit code 2",
			exitCode: 2,
			want: want{
				status: preview.StackStatusChanged,
			},
		},
		{
			name:     "exit code other",
			exitCode: 9,
			want: want{
				status: preview.StackStatusFailed,
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			status := preview.DerivePreviewStatus(tc.exitCode)
			assert.EqualStrings(t, tc.want.status.String(), status.String(), "unexpected status")
		})
	}
}
