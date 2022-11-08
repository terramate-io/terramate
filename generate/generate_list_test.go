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

package generate_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/generate"
	"github.com/mineiros-io/terramate/generate/genhcl"
	"github.com/mineiros-io/terramate/test/sandbox"
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
			name: "single generated file on root",
			layout: []string{
				genFile("generated.tf"),
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
				genFile("generated.tf", "data"),
			},
			want: []string{"generated.tf"},
		},
		{
			name: "multiple generated files",
			layout: []string{
				genFile("generated1.tf"),
				genFile("generated2.hcl"),
				genFile("somename"),
			},
			want: []string{"generated1.tf", "generated2.hcl", "somename"},
		},
		{
			name: "multiple generated files mixed versions",
			layout: []string{
				"f:old.tf:" + genhcl.HeaderV0,
				genFile("current.hcl"),
			},
			want: []string{"current.hcl", "old.tf"},
		},
		{
			name: "gen and manual files mixed",
			layout: []string{
				genFile("gen.tf"),
				"f:manual.tf:some terraform stuff",
				genFile("gen2.tf"),
				"f:manual2.tf:data",
			},
			want: []string{"gen.tf", "gen2.tf"},
		},
		{
			name: "on root ignores generated files inside dir with .tmskip",
			layout: []string{
				genFile("genfiles/1.tf"),
				genFile("genfiles/2.tf"),
				genFile("genfiles/" + config.SkipFilename),
				genFile("genfiles/subdir/1.tf"),
				genFile("genfiles2/1.tf"),
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
				genFile("stack/1.tf"),
				genFile("stack/dir/" + config.SkipFilename),
				genFile("stack/dir/1.tf"),
			},
			want: []string{
				"1.tf",
			},
		},
		{
			name: "on root lists all generated files except inside stacks",
			layout: []string{
				genFile("genfiles/1.tf"),
				genFile("genfiles/2.tf"),
				genFile("genfiles2/1.tf"),
				genFile("dir/sub/genfiles/1.tf"),
				"s:stack-1",
				genFile("stack-1/1.tf"),
				"s:stack-2",
				genFile("stack-2/1.tf"),
				"s:stack-1/substack",
				genFile("stack-1/substack/1.tf"),
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
				genFile("stack/1.tf"),
				genFile("stack/2.tf"),
				genFile("stack/subdir/1.tf"),
				genFile("stack/subdir2/1.tf"),
				genFile("stack/sub/dir/1.tf"),
				"s:stack/child",
				genFile("stack/child/1.tf"),
				genFile("stack/child/dir/1.tf"),
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
			name: "ignores dot dirs and files",
			layout: []string{
				genFile(".name.tf"),
				genFile(".dir/1.tf"),
				genFile(".dir/2.tf"),
			},
			want: []string{},
		},
	}

	for _, tc := range tcases {
		tcase := tc
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			s := sandbox.New(t)
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

func genFile(path string, body ...string) string {
	return fmt.Sprintf("f:%s:%s\n%s", path, genhcl.Header, strings.Join(body, ""))
}
