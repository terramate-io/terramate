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
		str  string
		want drift.Status
	}
	for _, tc := range []testcase{
		{str: "ok", want: drift.OK},
		{str: "unknown", want: drift.Unknown},
		{str: "failed", want: drift.Failed},
		{str: "drifted", want: drift.Drifted},
		{str: "okay", want: drift.Unrecognized},
		{str: "OK", want: drift.Unrecognized},
		{str: "Ok", want: drift.Unrecognized},
		{str: "Drifted", want: drift.Unrecognized},
		{str: "", want: drift.Unrecognized},
	} {
		tc := tc
		t.Run(fmt.Sprintf("drift status %s", tc.str), func(t *testing.T) {
			t.Parallel()
			got := drift.NewStatus(tc.str)
			if got != tc.want {
				t.Fatalf("unexpected status: %s != %s", tc.want, got)
			}
			if tc.want == drift.Unrecognized {
				return
			}
			assert.EqualStrings(t, got.String(), tc.str)
			assert.NoError(t, got.Validate())
			var s drift.Status
			assert.NoError(t, s.UnmarshalJSON([]byte(strconv.Quote(tc.str))))
			assert.NoError(t, s.Validate())
			marshaled, err := s.MarshalJSON()
			assert.NoError(t, err)
			assert.EqualStrings(t, strconv.Quote(tc.str), string(marshaled))
		})
	}
}
