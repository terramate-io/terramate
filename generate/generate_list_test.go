// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package generate_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/generate"
	"github.com/terramate-io/terramate/generate/genhcl"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestGeneratedFilesListing(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name   string
		layout []string
		dir    string
		want   []string
	}

	tcases := []testcase{
		{
			name: "no files equals empty",
		},
		{
			name: "single file, non-generated equals empty",
			layout: []string{
				"f:somefile.tf:whatever",
			},
		},
		{
			name: "single empty file equals empty",
			layout: []string{
				"f:somefile.tf",
			},
		},
		{
			name: "multiple files, multiple suffixes, non-generated equals empty",
			layout: []string{
				"f:file.tf:whatever",
				"f:file.hcl:dont care",
				"f:another.tm.hcl:terramate {}",
			},
		},
		{
			name: "single file generated but configured to a different comment style",
			layout: []string{
				"f:somefile.tf:" + genhcl.Header(genhcl.SlashComment) + "test",
				"f:terramate.tm:" + Doc(Terramate(
					Config(
						Block("generate", Doc(
							Str("hcl_magic_header_comment_style", "#"),
						)),
					),
				)).String(),
			},
		},
		{
			name: "single file generated and properly configured with same comment style",
			layout: []string{
				"f:somefile.tf:" + genhcl.Header(genhcl.HashComment) + "test",
				"f:terramate.tm:" + Doc(Terramate(
					Config(
						Block("generate", Doc(
							Str("hcl_magic_header_comment_style", "#"),
						)),
					),
				)).String(),
			},
			want: []string{"somefile.tf"},
		},
		{
			name: "single generated file on root",
			layout: []string{
				genfile("generated.tf"),
			},
			want: []string{"generated.tf"},
		},
		{
			name: "single generated file, v0 header detection",
			layout: []string{
				"f:generated.tf:" + genhcl.HeaderV0,
			},
			want: []string{"generated.tf"},
		},
		{
			name: "single generated file contents after header newline dont matter",
			layout: []string{
				genfile("generated.tf", "data"),
			},
			want: []string{"generated.tf"},
		},
		{
			name: "multiple generated files",
			layout: []string{
				genfile("generated1.tf"),
				genfile("generated2.hcl"),
				genfile("somename"),
			},
			want: []string{"generated1.tf", "generated2.hcl", "somename"},
		},
		{
			name: "multiple generated files mixed versions",
			layout: []string{
				"f:old.tf:" + genhcl.HeaderV0,
				genfile("current.hcl"),
			},
			want: []string{"current.hcl", "old.tf"},
		},
		{
			name: "gen and manual files mixed",
			layout: []string{
				genfile("gen.tf"),
				"f:manual.tf:some terraform stuff",
				genfile("gen2.tf"),
				"f:manual2.tf:data",
			},
			want: []string{"gen.tf", "gen2.tf"},
		},
		{
			name: "on root ignores generated files inside dir with .tmskip",
			layout: []string{
				genfile("genfiles/1.tf"),
				genfile("genfiles/2.tf"),
				genfile("genfiles/" + config.SkipFilename),
				genfile("genfiles/subdir/1.tf"),
				genfile("genfiles2/1.tf"),
			},
			want: []string{
				"genfiles2/1.tf",
			},
		},
		{
			name: "on stack ignores generated files inside dir with .tmskip",
			dir:  "stack",
			layout: []string{
				"s:stack",
				genfile("stack/1.tf"),
				genfile("stack/dir/" + config.SkipFilename),
				genfile("stack/dir/1.tf"),
			},
			want: []string{
				"1.tf",
			},
		},
		{
			name: "on root lists all generated files except inside stacks",
			layout: []string{
				genfile("genfiles/1.tf"),
				genfile("genfiles/2.tf"),
				genfile("genfiles2/1.tf"),
				genfile("dir/sub/genfiles/1.tf"),
				"s:stack-1",
				genfile("stack-1/1.tf"),
				"s:stack-2",
				genfile("stack-2/1.tf"),
				"s:stack-1/substack",
				genfile("stack-1/substack/1.tf"),
			},
			want: []string{
				"genfiles/1.tf",
				"genfiles/2.tf",
				"genfiles2/1.tf",
				"dir/sub/genfiles/1.tf",
			},
		},
		{
			name: "on stack lists all generated files except inside child stacks",
			dir:  "stack",
			layout: []string{
				"s:stack",
				genfile("stack/1.tf"),
				genfile("stack/2.tf"),
				genfile("stack/subdir/1.tf"),
				genfile("stack/subdir2/1.tf"),
				genfile("stack/sub/dir/1.tf"),
				"s:stack/child",
				genfile("stack/child/1.tf"),
				genfile("stack/child/dir/1.tf"),
			},
			want: []string{
				"1.tf",
				"2.tf",
				"subdir/1.tf",
				"subdir2/1.tf",
				"sub/dir/1.tf",
			},
		},
		{
			name: "ignores dot dirs",
			layout: []string{
				genfile(".dir/1.tf"),
				genfile(".dir/2.tf"),
			},
			want: []string{},
		},
		{
			// https://github.com/terramate-io/terramate/issues/1260
			name: "regression test: dotfiles should not be ignored if not inside a .tmskip",
			layout: []string{
				genfile(".name.tf"),
				genfile("dir/.name.tf"),
			},
			want: []string{
				".name.tf",
				"dir/.name.tf",
			},
		},
	}

	for _, tc := range tcases {
		tcase := tc
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			s := sandbox.NoGit(t, true)
			s.BuildTree(tcase.layout)

			var listdir string
			if tcase.dir != "" {
				listdir = filepath.Join(s.RootDir(), tcase.dir)
			} else {
				listdir = s.RootDir()
			}

			got, err := generate.ListGenFiles(s.Config(), listdir)
			assert.NoError(t, err)
			assertEqualStringList(t, got, tcase.want)
		})
	}
}

func genfile(path string, body ...string) string {
	return fmt.Sprintf("f:%s:%s\n%s", path, genhcl.DefaultHeader(), strings.Join(body, ""))
}
