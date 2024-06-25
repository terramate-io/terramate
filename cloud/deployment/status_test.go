// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package deployment_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cloud/deployment"
	"github.com/terramate-io/terramate/errors"
	errtest "github.com/terramate-io/terramate/test/errors"
)

func TestDeploymentStatus(t *testing.T) {
	t.Parallel()

	var s deployment.Status
	errtest.Assert(t, s.Validate(), errors.E(deployment.ErrInvalidStatus))
	s = deployment.Canceled + 1
	errtest.Assert(t, s.Validate(), errors.E(deployment.ErrInvalidStatus))

	type testcase struct {
		str  string
		want deployment.Status
	}
	for _, tc := range []testcase{
		{str: "pending", want: deployment.Pending},
		{str: "running", want: deployment.Running},
		{str: "ok", want: deployment.OK},
		{str: "failed", want: deployment.Failed},
		{str: "canceled", want: deployment.Canceled},
		{str: "okay", want: deployment.Unrecognized},
		{str: "OK", want: deployment.Unrecognized},
		{str: "Ok", want: deployment.Unrecognized},
	} {
		tc := tc
		t.Run(fmt.Sprintf("deployment %s", tc.str), func(t *testing.T) {
			t.Parallel()
			got := deployment.NewStatus(tc.str)
			if got != tc.want {
				t.Fatalf("%s != %s", got, tc.want)
			}
			if tc.want == deployment.Unrecognized {
				return
			}
			assert.EqualStrings(t, got.String(), tc.str)
			assert.NoError(t, got.Validate())
			var s deployment.Status
			assert.NoError(t, s.UnmarshalJSON([]byte(strconv.Quote(tc.str))))
			assert.NoError(t, s.Validate())
			marshaled, err := s.MarshalJSON()
			assert.NoError(t, err)
			assert.EqualStrings(t, strconv.Quote(tc.str), string(marshaled))
		})
	}
}
