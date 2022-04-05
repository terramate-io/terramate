// Copyright 2021 Mineiros GmbH
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

package terramate_test

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/stack"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestLoadMetadata(t *testing.T) {

	type testcase struct {
		name    string
		layout  []string
		want    []stack.Metadata
		wantErr error
	}

	const invalidHCL = "block {"

	tcases := []testcase{
		{
			name:   "no stacks",
			layout: []string{},
			want:   []stack.Metadata{},
		},
		{
			name:   "single stacks",
			layout: []string{"s:stack"},
			want: []stack.Metadata{
				{
					Name: "stack",
					Path: "/stack",
				},
			},
		},
		{
			name: "two stacks",
			layout: []string{
				"s:stack-1",
				"s:stack-2",
			},
			want: []stack.Metadata{
				{
					Name: "stack-1",
					Path: "/stack-1",
				},
				{
					Name: "stack-2",
					Path: "/stack-2",
				},
			},
		},
		{
			name: "stack and some non-stack dirs",
			layout: []string{
				"s:stack",
				"d:non-stack",
				"d:non-stack-2",
			},
			want: []stack.Metadata{
				{
					Name: "stack",
					Path: "/stack",
				},
			},
		},
		{
			name: "stacks nested",
			layout: []string{
				"s:envs/prod/stack-1",
				"s:envs/staging/stack-1",
			},
			want: []stack.Metadata{
				{
					Name: "stack-1",
					Path: "/envs/prod/stack-1",
				},
				{
					Name: "stack-1",
					Path: "/envs/staging/stack-1",
				},
			},
		},
		{
			name: "stacks with description",
			layout: []string{
				"s:stack:description=desc",
			},
			want: []stack.Metadata{
				{
					Name:        "stack",
					Path:        "/stack",
					Description: "desc",
				},
			},
		},
		{
			name: "single invalid stack",
			layout: []string{
				fmt.Sprintf("f:invalid-stack/%s:data=%s", config.DefaultFilename, invalidHCL),
			},
			wantErr: errors.E(errors.HCLSyntax),
		},
		{
			name: "valid stack with invalid stack",
			layout: []string{
				"s:stack-valid-1",
				fmt.Sprintf("f:invalid-stack/%s:data=%s", config.DefaultFilename, invalidHCL),
			},
			wantErr: errors.E(errors.HCLSyntax),
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tcase.layout)

			stackEntries, err := terramate.ListStacks(s.RootDir())
			if tcase.wantErr != nil {
				errors.AssertKind(t, err, tcase.wantErr)
			}

			gotMetadata := []stack.Metadata{}
			for _, entry := range stackEntries {
				gotMetadata = append(gotMetadata, entry.Stack.Meta())
			}

			assert.EqualInts(t, len(tcase.want), len(gotMetadata))

			for i, wantMeta := range tcase.want {
				if diff := cmp.Diff(wantMeta, gotMetadata[i], cmpopts.IgnoreUnexported(wantMeta)); diff != "" {
					t.Fatalf("want %+v != got %+v.\ndiff:\n%s", wantMeta, gotMetadata[i], diff)
				}
			}
		})
	}
}
