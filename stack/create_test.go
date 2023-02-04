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
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/stack"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestStackCreation(t *testing.T) {
	type wantedStack struct {
		id      string
		name    string
		desc    string
		imports []string
		after   []string
		before  []string
	}
	type want struct {
		err   error
		stack wantedStack
	}
	type testcase struct {
		name    string
		layout  []string
		stack   config.Stack
		imports []string
		want    want
	}

	newID := func(id string) string {
		err := hcl.ValidateStackID(id)
		assert.NoError(t, err)
		return id
	}

	testcases := []testcase{
		{
			name:  "default create configuration",
			stack: config.Stack{Dir: project.NewPath("/stack")},
			want: want{
				stack: wantedStack{name: "stack"},
			},
		},
		{
			name:  "creates all dirs no stack path",
			stack: config.Stack{Dir: project.NewPath("/dir1/dir2/dir3/stack")},
			want: want{
				stack: wantedStack{name: "stack"},
			},
		},
		{
			name:   "creates configuration when dir already exists",
			layout: []string{"d:stack"},
			stack:  config.Stack{Dir: project.NewPath("/stack")},
			want: want{
				stack: wantedStack{name: "stack"},
			},
		},
		{
			name:   "creates configuration when dir already exists and has subdirs",
			layout: []string{"d:stack/subdir"},
			stack:  config.Stack{Dir: project.NewPath("/stack")},
			want: want{
				stack: wantedStack{name: "stack"},
			},
		},
		{
			name: "defining only name",
			stack: config.Stack{
				Dir:  project.NewPath("/another-stack"),
				Name: "The Name Of The Stack",
			},
			want: want{
				stack: wantedStack{
					name: "The Name Of The Stack",
				},
			},
		},
		{
			name: "defining only description",
			stack: config.Stack{
				Dir:         project.NewPath("/cool-stack"),
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
			stack: config.Stack{
				Dir:         project.NewPath("/stack"),
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
		{
			name:    "defining single import",
			stack:   config.Stack{Dir: project.NewPath("/stack-imports")},
			imports: []string{"/common/something.tm.hcl"},
			want: want{
				stack: wantedStack{
					name:    "stack-imports",
					imports: []string{"/common/something.tm.hcl"},
				},
			},
		},
		{
			name:  "defining multiple imports",
			stack: config.Stack{Dir: project.NewPath("/stack-imports")},
			imports: []string{
				"/common/1.tm.hcl",
				"/common/2.tm.hcl",
			},
			want: want{
				stack: wantedStack{
					name: "stack-imports",
					imports: []string{
						"/common/1.tm.hcl",
						"/common/2.tm.hcl",
					},
				},
			},
		},
		{
			name: "defining after/before",
			stack: config.Stack{
				Dir:    project.NewPath("/stack-after-before"),
				After:  []string{"stack-1", "stack-2"},
				Before: []string{"stack-3", "stack-4"},
			},
			want: want{
				stack: wantedStack{
					name:   "stack-after-before",
					after:  []string{"stack-1", "stack-2"},
					before: []string{"stack-3", "stack-4"},
				},
			},
		},
		{
			name: "fails on invalid stack ID",
			stack: config.Stack{
				Dir: project.NewPath("/stack"),
				ID:  "not valid ID",
			},
			want: want{err: errors.E(stack.ErrInvalidStackID)},
		},
		{
			name:  "dotdir is not allowed as stack dir",
			stack: config.Stack{Dir: project.NewPath("/.stack")},
			want:  want{err: errors.E(stack.ErrInvalidStackDir)},
		},
		{
			name:  "dotdir is not allowed as stack dir as subdir",
			stack: config.Stack{Dir: project.NewPath("/stacks/.stack")},
			want:  want{err: errors.E(stack.ErrInvalidStackDir)},
		},
		{
			name:   "fails if stack already exists",
			layout: []string{"f:stack/config.tm:stack{\n}"},
			stack:  config.Stack{Dir: project.NewPath("/stack")},
			want:   want{err: errors.E(stack.ErrStackAlreadyExists)},
		},
		{
			name:   "fails if there is a stack.tm.hcl file on dir",
			layout: []string{"f:stack/stack.tm.hcl"},
			stack:  config.Stack{Dir: project.NewPath("/stack")},
			want:   want{err: errors.E(stack.ErrStackDefaultCfgFound)},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tc.layout)
			buildImportedFiles(t, s.RootDir(), tc.imports)

			root, err := config.LoadRoot(s.RootDir())
			if errors.IsAnyKind(tc.want.err, hcl.ErrHCLSyntax, hcl.ErrTerramateSchema) {
				assert.IsError(t, err, tc.want.err)
				return
			}

			assert.NoError(t, err)
			err = stack.Create(root, tc.stack, tc.imports...)
			assert.IsError(t, err, tc.want.err)
			if tc.want.err != nil {
				return
			}

			want := tc.want.stack
			got := s.LoadStack(tc.stack.Dir)

			if want.id != "" {
				assert.EqualStrings(t, want.id, got.ID)
			} else if got.ID != "" {
				t.Fatalf("got unwanted ID %q", got.ID)
			}

			assert.EqualStrings(t, want.name, got.Name, "checking stack name")
			assert.EqualStrings(t, want.desc, got.Description, "checking stack description")

			test.AssertStackImports(t, s.RootDir(), got.HostDir(root), want.imports)
			test.AssertDiff(t, got.After, want.after, "created stack has invalid after")
			test.AssertDiff(t, got.Before, want.before, "created stack has invalid before")
		})
	}
}

func buildImportedFiles(t *testing.T, rootdir string, imports []string) {
	t.Helper()

	for _, importPath := range imports {
		abspath := filepath.Join(rootdir, importPath)
		test.WriteFile(t, filepath.Dir(abspath), filepath.Base(abspath), "")
	}
}
