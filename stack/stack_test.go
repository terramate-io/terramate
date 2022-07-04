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
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/stack"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/sandbox"
	"github.com/mineiros-io/terramate/tf"
)

func TestLoadAllFailsIfStacksIDIsNotUnique(t *testing.T) {
	s := sandbox.New(t)
	s.BuildTree([]string{
		"s:stacks/stack-1:id=id",
		"s:stacks/stack-2:id=id",
	})
	_, err := stack.LoadAll(s.RootDir())
	assert.IsError(t, err, errors.E(stack.ErrDuplicatedID))
}

func TestStackImplicitTFWatchFiles(t *testing.T) {
	type want struct {
		err   error
		watch []string
	}
	type tffile struct {
		path    string
		content string
		modules []tf.Module
	}
	type testcase struct {
		name      string
		layout    []string
		tfFiles   []tffile
		stackpath string
		want      want
	}

	for _, tc := range []testcase{
		{
			name: "no modules",
			layout: []string{
				"s:stack",
			},
			stackpath: "stack",
		},
		{
			name: "1 module dependency",
			layout: []string{
				"s:stack",
			},
			stackpath: "stack",
			tfFiles: []tffile{
				{
					path:    "modules/mod/1.tf",
					content: `# empty module`,
				},
				{
					path: "stack/main.tf",
					modules: []tf.Module{
						{
							Source: "../modules/mod",
						},
					},
				},
			},
			want: want{
				watch: []string{
					"/modules/mod/1.tf",
				},
			},
		},
		{
			name: "3 module dependencies in the same file",
			layout: []string{
				"s:stack",
			},
			stackpath: "stack",
			tfFiles: []tffile{
				{
					path:    "modules/mod1/1.tf",
					content: `# empty module`,
				},
				{
					path:    "modules/mod2/2.tf",
					content: `# empty module`,
				},
				{
					path:    "modules/mod3/3.tf",
					content: `# empty module`,
				},
				{
					path: "stack/main.tf",
					modules: []tf.Module{
						{
							Source: "../modules/mod1",
						},
						{
							Source: "../modules/mod2",
						},
						{
							Source: "../modules/mod3",
						},
					},
				},
			},
			want: want{
				watch: []string{
					"/modules/mod1/1.tf",
					"/modules/mod2/2.tf",
					"/modules/mod3/3.tf",
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tc.layout)

			for _, tfFile := range tc.tfFiles {
				var content []string
				for i, mod := range tfFile.modules {
					content = append(content, fmt.Sprintf(`
						module "something-%d" {
							source = %q
						}
					`, i, mod.Source))
				}

				name := filepath.Base(tfFile.path)
				dir := filepath.Join(s.RootDir(), filepath.Dir(tfFile.path))
				test.AppendFile(t, dir, name, tfFile.content)
				test.AppendFile(t, dir, name, strings.Join(content, "\n\n"))
			}

			st, err := stack.Load(s.RootDir(), filepath.Join(s.RootDir(), tc.stackpath))
			assert.NoError(t, err)

			watch, err := st.Watch()
			assert.NoError(t, err)
			assertSameWatchFiles(t, watch, tc.want.watch)
		})
	}
}

func assertSameWatchFiles(t *testing.T, want, got []string) {
	assert.EqualInts(t, len(want), len(got))
	for i, wantFile := range want {
		assert.EqualStrings(t, wantFile, got[i])
	}
}
