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

package genfile_test

import (
	"testing"

	. "github.com/mineiros-io/terramate/test/hclutils"
	. "github.com/mineiros-io/terramate/test/hclutils/info"
	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
)

func TestGenerateFileRange(t *testing.T) {
	t.Parallel()

	tcases := []testcase{
		{
			name:   "multiple blocks",
			layout: []string{"s:stack"},
			dir:    "/stack",
			configs: []hclconfig{
				{
					path: "/stack/generate.tm",
					add: Doc(
						GenerateFile(
							Labels("code.hcl"),
							Str("content", "data"),
						),
						GenerateFile(
							Labels("code2.hcl"),
							Str("content", "data"),
						),
					),
				},
			},
			want: []result{
				{
					name: "code.hcl",
					file: genFile{
						origin: Range(
							"stack/generate.tm",
							Start(2, 1, 1),
							End(4, 2, 48),
						),
						condition: true,
						body:      "data",
					},
				},
				{
					name: "code2.hcl",
					file: genFile{
						origin: Range(
							"stack/generate.tm",
							Start(5, 1, 49),
							End(7, 2, 97),
						),
						condition: true,
						body:      "data",
					},
				},
			},
		},
		{
			name:   "multiple files",
			layout: []string{"s:stack"},
			dir:    "/stack",
			configs: []hclconfig{
				{
					path: "/stack/generate.tm",
					add: GenerateFile(
						Labels("code.hcl"),
						Str("content", "data"),
					),
				},
				{
					path: "/stack/generate2.tm",
					add: GenerateFile(
						Labels("code2.hcl"),
						Str("content", "data"),
					),
				},
			},
			want: []result{
				{
					name: "code.hcl",
					file: genFile{
						origin: Range(
							"stack/generate.tm",
							Start(2, 1, 1),
							End(4, 2, 48),
						),
						condition: true,
						body:      "data",
					},
				},
				{
					name: "code2.hcl",
					file: genFile{
						origin: Range(
							"stack/generate2.tm",
							Start(2, 1, 1),
							End(4, 2, 49),
						),
						condition: true,
						body:      "data",
					},
				},
			},
		},
	}

	for _, tcase := range tcases {
		testGenfile(t, tcase)
	}
}
