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
		str string
		err error
	}
	for _, tc := range []testcase{
		{str: "pending"},
		{str: "running"},
		{str: "ok"},
		{str: "failed"},
		{str: "canceled"},
		{
			str: "okay",
			err: errors.E(deployment.ErrInvalidStatus),
		},
		{
			str: "OK",
			err: errors.E(deployment.ErrInvalidStatus),
		},
		{
			str: "Ok",
			err: errors.E(deployment.ErrInvalidStatus),
		},
	} {
		tc := tc
		t.Run(fmt.Sprintf("deployment %s", tc.str), func(t *testing.T) {
			t.Parallel()
			s, err := deployment.NewStatus(tc.str)
			errtest.Assert(t, err, tc.err)
			if err == nil {
				assert.EqualStrings(t, s.String(), tc.str)
				assert.NoError(t, s.Validate())
				var s deployment.Status
				assert.NoError(t, s.UnmarshalJSON([]byte(strconv.Quote(tc.str))))
				assert.NoError(t, s.Validate())
				marshaled, err := s.MarshalJSON()
				assert.NoError(t, err)
				assert.EqualStrings(t, strconv.Quote(tc.str), string(marshaled))
			}
		})
	}
}
