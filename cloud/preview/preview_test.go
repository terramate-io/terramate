// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package preview_test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cloud/preview"
)

func TestDerivePreviewStatus(t *testing.T) {
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
