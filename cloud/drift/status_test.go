// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package drift_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cloud/drift"
	"github.com/terramate-io/terramate/errors"
	errtest "github.com/terramate-io/terramate/test/errors"
)

func TestDriftStatus(t *testing.T) {
	t.Parallel()

	var s drift.Status
	errtest.Assert(t, s.Validate(), errors.E(drift.ErrInvalidStatus))
	s = drift.Failed + 1
	errtest.Assert(t, s.Validate(), errors.E(drift.ErrInvalidStatus))

	type testcase struct {
		str string
		err error
	}
	for _, tc := range []testcase{
		{str: "ok"},
		{str: "unknown"},
		{str: "failed"},
		{str: "drifted"},
		{
			str: "okay",
			err: errors.E(drift.ErrInvalidStatus),
		},
		{
			str: "OK",
			err: errors.E(drift.ErrInvalidStatus),
		},
		{
			str: "Ok",
			err: errors.E(drift.ErrInvalidStatus),
		},
		{
			str: "Drifted",
			err: errors.E(drift.ErrInvalidStatus),
		},
		{
			str: "",
			err: errors.E(drift.ErrInvalidStatus),
		},
	} {
		tc := tc
		t.Run(fmt.Sprintf("drift status %s", tc.str), func(t *testing.T) {
			t.Parallel()
			s, err := drift.NewStatus(tc.str)
			errtest.Assert(t, err, tc.err)
			if err == nil {
				assert.EqualStrings(t, s.String(), tc.str)
				assert.NoError(t, s.Validate())
				var s drift.Status
				assert.NoError(t, s.UnmarshalJSON([]byte(strconv.Quote(tc.str))))
				assert.NoError(t, s.Validate())
				marshaled, err := s.MarshalJSON()
				assert.NoError(t, err)
				assert.EqualStrings(t, strconv.Quote(tc.str), string(marshaled))
			}
		})
	}
}
