// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stack_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cloud/stack"
	"github.com/terramate-io/terramate/errors"
	errtest "github.com/terramate-io/terramate/test/errors"
)

func TestStackStatus(t *testing.T) {
	t.Parallel()

	var s stack.Status
	errtest.Assert(t, s.Validate(), errors.E(stack.ErrInvalidStatus))
	s = stack.Failed + 1
	errtest.Assert(t, s.Validate(), errors.E(stack.ErrInvalidStatus))

	type testcase struct {
		str  string
		want stack.Status
	}
	for _, tc := range []testcase{
		{str: "ok", want: stack.OK},
		{str: "drifted", want: stack.Drifted},
		{str: "okay", want: stack.Unrecognized},
		{str: "OK", want: stack.Unrecognized},
		{str: "Ok", want: stack.Unrecognized},
	} {
		tc := tc
		t.Run(fmt.Sprintf("status %s", tc.str), func(t *testing.T) {
			t.Parallel()
			s := stack.NewStatus(tc.str)
			assert.EqualInts(t, int(s), int(tc.want))
			if s != stack.Unrecognized {
				assert.EqualStrings(t, s.String(), tc.str)
			}
			assert.NoError(t, s.Validate())
			var s2 stack.Status
			assert.NoError(t, s2.UnmarshalJSON([]byte(strconv.Quote(tc.str))))
			assert.NoError(t, s2.Validate())

			if s2 != stack.Unrecognized {
				marshaled, err := s2.MarshalJSON()
				assert.NoError(t, err)
				assert.EqualStrings(t, strconv.Quote(tc.str), string(marshaled))
			}
		})
	}
}
