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

package genhcl_test

import (
	"testing"

	. "github.com/mineiros-io/terramate/test/hclutils"
	. "github.com/mineiros-io/terramate/test/hclutils/info"
	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
)

func TestGenerateHCLRange(t *testing.T) {
	t.Parallel()

	tcases := []testcase{
		{
			name:  "multiple blocks",
			stack: "/stack",
			configs: []hclconfig{
				{
					path:     "/stack",
					filename: "generate.tm",
					add: Doc(
						GenerateHCL(
							Labels("code.hcl"),
							Content(
								Str("a", "data"),
							),
						),
						GenerateHCL(
							Labels("code2.hcl"),
							Content(
								Str("a", "data"),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "code.hcl",
					hcl: genHCL{
						origin: Range(
							"stack/generate.tm",
							Start(2, 1, 1),
							End(6, 2, 59),
						),
						condition: true,
						body: Doc(
							Str("a", "data"),
						),
					},
				},
				{
					name: "code2.hcl",
					hcl: genHCL{
						origin: Range(
							"stack/generate.tm",
							Start(7, 1, 60),
							End(11, 2, 119),
						),
						condition: true,
						body: Doc(
							Str("a", "data"),
						),
					},
				},
			},
		},
		{
			name:  "multiple files",
			stack: "/stack",
			configs: []hclconfig{
				{
					path:     "/stack",
					filename: "generate.tm",
					add: GenerateHCL(
						Labels("code.hcl"),
						Content(
							Str("a", "data"),
						),
					),
				},
				{
					path:     "/stack",
					filename: "generate2.tm",
					add: GenerateHCL(
						Labels("code2.hcl"),
						Content(
							Str("a", "data"),
						),
					),
				},
			},
			want: []result{
				{
					name: "code.hcl",
					hcl: genHCL{
						origin: Range(
							"stack/generate.tm",
							Start(2, 1, 1),
							End(6, 2, 59),
						),
						condition: true,
						body: Doc(
							Str("a", "data"),
						),
					},
				},
				{
					name: "code2.hcl",
					hcl: genHCL{
						origin: Range(
							"stack/generate2.tm",
							Start(2, 1, 1),
							End(6, 2, 60),
						),
						condition: true,
						body: Doc(
							Str("a", "data"),
						),
					},
				},
			},
		},
	}

	for _, tcase := range tcases {
		tcase.run(t)
	}
}
