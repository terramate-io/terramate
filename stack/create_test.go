// Copyright 2022 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package stack_test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/stack"
	"github.com/mineiros-io/terramate/test/errors"
	"github.com/mineiros-io/terramate/test/sandbox"
)

// TODO(katcipis)
//
// - Dot dir is not allowed
// - Dir outside project
// - Dir exists
// - Dir + stack.tm.hcl exists (no stack config, only file)
// - Dir already exists and dirs inside are not stack
// - Dir with stack already exists (file is not stack.tm.hcl)
// - Dir with other configs but no stack definition and no stack.tm.hcl

func TestStackCreation(t *testing.T) {
	type wantedStack struct {
		name string
		desc string
	}
	type want struct {
		err   error
		stack wantedStack
	}
	type testcase struct {
		name   string
		layout []string
		create stack.CreateCfg
		want   want
	}

	testcases := []testcase{
		{
			name: "default create configuration",
			create: stack.CreateCfg{
				Dir: "stack",
			},
			want: want{
				stack: wantedStack{
					name: "stack",
					desc: "stack",
				},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tc.layout)
			err := stack.Create(s.RootDir(), tc.create)
			errors.Assert(t, err, tc.want.err)

			if tc.want.err != nil {
				return
			}

			want := tc.want.stack
			got := s.LoadStack(tc.create.Dir)

			assert.EqualStrings(t, want.name, got.Name())
			assert.EqualStrings(t, want.desc, got.Desc())
		})
	}
}
