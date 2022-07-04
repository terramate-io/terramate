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

	"github.com/google/uuid"
	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/stack"
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
		id   hcl.StackID
		name string
		desc string
	}
	type want struct {
		err   bool
		stack wantedStack
	}
	type testcase struct {
		name   string
		layout []string
		create stack.CreateCfg
		want   want
	}

	newID := func(id string) hcl.StackID {
		sid, err := hcl.NewStackID(id)
		assert.NoError(t, err)
		return sid
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
		{
			name: "defining only name",
			create: stack.CreateCfg{
				Dir:  "another-stack",
				Name: "The Name Of The Stack",
			},
			want: want{
				stack: wantedStack{
					name: "The Name Of The Stack",
					desc: "The Name Of The Stack",
				},
			},
		},
		{
			name: "defining only description",
			create: stack.CreateCfg{
				Dir:         "cool-stack",
				Description: "Stack Description",
			},
			want: want{
				stack: wantedStack{
					name: "cool-stack",
					desc: "Stack Description",
				},
			},
		},
		{
			name: "defining ID/name/description",
			create: stack.CreateCfg{
				Dir:         "stack",
				ID:          "stack-id",
				Name:        "Stack Name",
				Description: "Stack Description",
			},
			want: want{
				stack: wantedStack{
					id:   newID("stack-id"),
					name: "Stack Name",
					desc: "Stack Description",
				},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tc.layout)
			err := stack.Create(s.RootDir(), tc.create)

			if tc.want.err {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			want := tc.want.stack
			got := s.LoadStack(tc.create.Dir)

			gotID, _ := got.ID()
			if wantID, ok := want.id.Value(); ok {
				assert.EqualStrings(t, wantID, gotID)
			} else {
				_, err := uuid.Parse(gotID)
				assert.NoError(t, err)
			}
			assert.EqualStrings(t, want.name, got.Name(), "checking stack name")
			assert.EqualStrings(t, want.desc, got.Desc(), "checking stack description")
		})
	}
}
