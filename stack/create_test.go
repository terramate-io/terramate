// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stack_test

import (
	"path"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/stack"
	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestStackCreation(t *testing.T) {
	t.Parallel()
	type want struct {
		err     error
		imports []string
	}
	type testcase struct {
		name    string
		layout  []string
		stack   config.Stack
		imports []string
		want    want
	}

	testcases := []testcase{
		{
			name:  "default create configuration",
			stack: config.Stack{Dir: project.NewPath("/stack")},
		},
		{
			name:  "creates all dirs no stack path",
			stack: config.Stack{Dir: project.NewPath("/dir1/dir2/dir3/stack")},
		},
		{
			name:   "creates configuration when dir already exists",
			layout: []string{"d:stack"},
			stack:  config.Stack{Dir: project.NewPath("/stack")},
		},
		{
			name:   "creates configuration when dir already exists and has subdirs",
			layout: []string{"d:stack/subdir"},
			stack:  config.Stack{Dir: project.NewPath("/stack")},
		},
		{
			name: "defining only name",
			stack: config.Stack{
				Dir:  project.NewPath("/another-stack"),
				Name: "The Name Of The Stack",
			},
		},
		{
			name: "defining only description",
			stack: config.Stack{
				Dir:         project.NewPath("/cool-stack"),
				Description: "Stack Description",
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
		},
		{
			name:    "defining single import",
			stack:   config.Stack{Dir: project.NewPath("/stack-imports")},
			imports: []string{"/common/something.tm.hcl"},
			want:    want{imports: []string{"/common/something.tm.hcl"}},
		},
		{
			name:  "defining multiple imports",
			stack: config.Stack{Dir: project.NewPath("/stack-imports")},
			imports: []string{
				"/common/1.tm.hcl",
				"/common/2.tm.hcl",
			},
			want: want{
				imports: []string{
					"/common/1.tm.hcl",
					"/common/2.tm.hcl",
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
		},
		{
			name: "defining tags",
			stack: config.Stack{
				Dir:  project.NewPath("/stack-with-tags"),
				Tags: []string{"a", "b"},
			},
		},
		{
			name: "defining duplicated tags - fails",
			stack: config.Stack{
				Dir:  project.NewPath("/stack-with-tags"),
				Tags: []string{"a", "a"},
			},
			want: want{
				err: errors.E(config.ErrStackValidation),
			},
		},
		{
			name: "defining invalid after",
			stack: config.Stack{
				Dir:   project.NewPath("/stack-after"),
				After: []string{"stack-1", "stack-1"},
			},
			want: want{err: errors.E(config.ErrStackValidation)},
		},
		{
			name: "defining invalid before",
			stack: config.Stack{
				Dir:    project.NewPath("/stack-before"),
				Before: []string{"stack-1", "stack-1"},
			},
			want: want{err: errors.E(config.ErrStackValidation)},
		},
		{
			name: "fails on invalid stack ID",
			stack: config.Stack{
				Dir: project.NewPath("/stack"),
				ID:  "not valid ID",
			},
			want: want{err: errors.E(config.ErrStackValidation)},
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
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := sandbox.NoGit(t, true)
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

			if tc.stack.Name == "" {
				tc.stack.Name = path.Base(tc.stack.Dir.String())
			}

			got := s.LoadStack(tc.stack.Dir)
			test.AssertStackImports(t, s.RootDir(), got.HostDir(root), tc.want.imports)
			test.AssertDiff(t, *got, tc.stack, "created stack is invalid")
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
