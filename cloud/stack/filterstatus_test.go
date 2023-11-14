// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stack_test

import (
	"fmt"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cloud/stack"
)

func TestNewStatus(t *testing.T) {
	t.Parallel()

	type testcase struct {
		status string
		want   stack.FilterStatus
	}
	for _, tc := range []testcase{
		{status: "ok", want: stack.FilterStatusOK},
		{status: "drifted", want: stack.FilterStatusDrifted},
		{status: "failed", want: stack.FilterStatusFailed},
		{status: "healthy", want: stack.FilterStatusHealthy},
		{status: "unhealthy", want: stack.FilterStatusUnhealthy},
		{status: "all", want: stack.FilterStatusAll},
		{status: "abc", want: stack.FilterStatusUnrecognized},
		{status: "", want: stack.NoFilter},
	} {
		tc := tc
		t.Run(fmt.Sprintf("status %s", tc.status), func(t *testing.T) {
			t.Parallel()
			s := stack.NewFilterStatus(tc.status)
			assert.EqualStrings(t, s.String(), tc.want.String())
		})
	}
}

func TestMetaEquals(t *testing.T) {
	t.Parallel()

	type testcase struct {
		filterStatus stack.FilterStatus
		status       string
		want         bool
	}
	for _, tc := range []testcase{
		{filterStatus: stack.NewFilterStatus("healthy"), status: "ok", want: true},
		{filterStatus: stack.NewFilterStatus("unhealthy"), status: "drifted", want: true},
		{filterStatus: stack.NewFilterStatus("unhealthy"), status: "failed", want: true},
		{filterStatus: stack.NewFilterStatus("all"), status: "failed", want: true},
		{filterStatus: stack.NewFilterStatus("all"), status: "ok", want: true},
		{filterStatus: stack.NewFilterStatus("all"), status: "drifted", want: true},
		{filterStatus: stack.NewFilterStatus("ok"), status: "ok", want: true},
		{filterStatus: stack.NewFilterStatus("ok"), status: "failed", want: false},
		{filterStatus: stack.NewFilterStatus("ok"), status: "abc123", want: false},
	} {
		tc := tc
		t.Run(fmt.Sprintf("(%s).MetaEquals(%s)", tc.filterStatus, tc.status), func(t *testing.T) {
			t.Parallel()
			if got := tc.filterStatus.MetaEquals(tc.status); got != tc.want {
				t.Fatalf("wrong result from MetaEquals, got %v, want %v", got, tc.want)
			}
		})
	}

}
