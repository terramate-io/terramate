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
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate"
	"github.com/mineiros-io/terramate/test/sandbox"
)

// TODO(katcipis):
//
// - on stack
// - on parent dir
// - on root dir
// - on stack + parent + root + no overriding
// - on stack + parent + root + overriding
// - using metadata
// - using tf functions
// - using metadata + tf functions

func TestLoadGlobals(t *testing.T) {

	type testcase struct {
		name   string
		layout []string
		want   terramate.StackGlobals
	}

	tcases := []testcase{
		{
			name:   "no stacks no globals",
			layout: []string{},
			want:   terramate.StackGlobals{},
		},
		{
			name:   "single stacks no globals",
			layout: []string{"s:stack"},
			want:   terramate.StackGlobals{},
		},
		{
			name: "two stacks no globals",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			want: terramate.StackGlobals{},
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tcase.layout)

			// TODO(katcipis): build config files

			for _, stackMetadata := range s.LoadMetadata().Stacks {
				got, err := terramate.LoadStackGlobals(s.BaseDir(), stackMetadata)
				assert.NoError(t, err)

				if !got.Equal(tcase.want) {
					t.Fatalf(
						"stack %q got:\n%v\nwant:\n%v\n",
						stackMetadata.Path,
						got,
						tcase.want,
					)
				}
			}
		})
	}
}
