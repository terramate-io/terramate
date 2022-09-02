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
	type (
		file struct {
			name string
			body string
		}
		testcase struct {
			name  string
			files []file
			want  []string
		}
	)

	tcases := []testcase{
		{
			name: "no files equals empty",
		},
		{
			name: "single file, non-generated equals empty",
			files: []file{
				{
					name: "somefile.tf",
					body: "whatever",
				},
			},
		},
		{
			name: "single empty file equals empty",
			files: []file{
				{
					name: "somefile.tf",
					body: "",
				},
			},
		},
		{
			name: "multiple files, multiple suffixes, non-generated equals empty",
			files: []file{
				{
					name: "file.tf",
					body: "whatever",
				},
				{
					name: "file.hcl",
					body: "dont care",
				},
				{
					name: "another.tm.hcl",
					body: "terramate is awesome",
				},
			},
		},
		{
			name: "single generated file, header detection",
			files: []file{
				{
					name: "generated.tf",
					body: genhcl.Header,
				},
			},
			want: []string{"generated.tf"},
		},
		{
			name: "single generated file, v0 header detection",
			files: []file{
				{
					name: "generated.tf",
					body: genhcl.HeaderV0,
				},
			},
			want: []string{"generated.tf"},
		},
		{
			name: "single generated file contents after header newline dont matter",
			files: []file{
				{
					name: "generated.tf",
					body: genhcl.Header + "\ndoesnt matter",
				},
			},
			want: []string{"generated.tf"},
		},
		{
			name: "multiple generated files",
			files: []file{
				{
					name: "generated1.tf",
					body: genhcl.Header,
				},
				{
					name: "generated2.hcl",
					body: genhcl.Header,
				},
				{
					name: "somename",
					body: genhcl.Header,
				},
			},
			want: []string{"generated1.tf", "generated2.hcl", "somename"},
		},
		{
			name: "multiple generated files mixed versions",
			files: []file{
				{
					name: "old.tf",
					body: genhcl.HeaderV0,
				},
				{
					name: "current.hcl",
					body: genhcl.Header,
				},
			},
			want: []string{"current.hcl", "old.tf"},
		},
		{
			name: "gen and manual files mixed",
			files: []file{
				{
					name: "gen.tf",
					body: genhcl.Header,
				},
				{
					name: "manual.tf",
					body: "some on terramate stuff",
				},
				{
					name: "gen2.tf",
					body: genhcl.Header,
				},
				{
					name: "manual2.tf",
					body: "data",
				},
			},
			want: []string{"gen.tf", "gen2.tf"},
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			s := sandbox.New(t)
			dirEntry := s.RootEntry().CreateDir("gen")

			for _, file := range tcase.files {
				dirEntry.CreateFile(file.name, file.body)
			}

			got, err := generate.ListGenFiles(dirEntry.Path())

			assert.NoError(t, err)
			assertEqualStringList(t, tcase.want, got)
		})
	}
}
