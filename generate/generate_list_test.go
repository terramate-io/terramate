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
	"testing"

	"github.com/madlambda/spells/assert"
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
				"f:another.tm.hcl:terramate is awesome",
			},
		},
		{
			name: "single generated file on root",
			layout: []string{
				"f:generated.tf:" + genhcl.Header,
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
				"f:generated.tf:" + genhcl.Header + "\ndoesnt matter",
			},
			want: []string{"generated.tf"},
		},
		{
			name: "multiple generated files",
			layout: []string{
				"f:generated1.tf:" + genhcl.Header,
				"f:generated2.hcl:" + genhcl.Header,
				"f:somename:" + genhcl.Header,
			},
			want: []string{"generated1.tf", "generated2.hcl", "somename"},
		},
		{
			name: "multiple generated files mixed versions",
			layout: []string{
				"f:old.tf:" + genhcl.HeaderV0,
				"f:current.hcl:" + genhcl.Header,
			},
			want: []string{"current.hcl", "old.tf"},
		},
		{
			name: "gen and manual files mixed",
			layout: []string{
				"f:gen.tf:" + genhcl.Header,
				"f:manual.tf:some terraform stuff",
				"f:gen2.tf:" + genhcl.Header,
				"f:manual2.tf:data",
			},
			want: []string{"gen.tf", "gen2.tf"},
		},
	}

	for _, tc := range tcases {
		tcase := tc
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			s := sandbox.New(t)
			s.BuildTree(tcase.layout)

			listdir := tcase.dir
			if listdir == "" {
				listdir = s.RootDir()
			}

			got, err := generate.ListGenFiles(listdir)

			assert.NoError(t, err)
			assertEqualStringList(t, got, tcase.want)
		})
	}
}
