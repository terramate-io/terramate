// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package fs_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/fs"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestListTerramateFiles(t *testing.T) {
	t.Parallel()

	type testcase struct {
		layout []string
		want   fs.ListResult
	}

	for _, tc := range []testcase{
		{
			layout: []string{
				"d:dir1",
				"d:dir2",
			},
			want: fs.ListResult{
				Dirs: []string{"dir1", "dir2"},
			},
		},
		{
			layout: []string{
				"f:test.txt",
				"f:test2.txt",
			},
			want: fs.ListResult{
				OtherFiles: []string{"test.txt", "test2.txt"},
			},
		},
		{
			layout: []string{
				"f:test.tm.hcl",
				"f:test2.tm",
				"f:test3.tmgen",
			},
			want: fs.ListResult{
				TmFiles:    []string{"test.tm.hcl", "test2.tm"},
				TmGenFiles: []string{"test3.tmgen"},
			},
		},
		{
			layout: []string{
				"f:.test.tm.hcl",
				"f:.tmskip",
			},
			want: fs.ListResult{
				Skipped: []string{".test.tm.hcl", ".tmskip"},
			},
		},
		{
			layout: []string{
				"f:test.tm.hcl",
				"f:test2.tm",
				"f:test3.tmgen",
				"f:test4.txt",
				"f:.test.txt",
				"f:dir/test.tm",
				"d:dir2",
				"f:dir3/.tmskip",
				"f:.tmskip",
			},
			want: fs.ListResult{
				TmFiles:    []string{"test.tm.hcl", "test2.tm"},
				TmGenFiles: []string{"test3.tmgen"},
				OtherFiles: []string{"test4.txt"},
				Dirs:       []string{"dir", "dir2", "dir3"},
				Skipped:    []string{".test.txt", ".tmskip"},
			},
		},
	} {
		tc := tc
		s := sandbox.NoGit(t, false)
		s.BuildTree(tc.layout)
		got, err := fs.ListTerramateFiles(s.RootDir())
		assert.NoError(t, err)
		if diff := cmp.Diff(tc.want, got); diff != "" {
			t.Errorf("ListTerramateFiles() result mismatch (-want +got):\n%s", diff)
		}
	}
}
