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

package hcl_test

import (
	"testing"

	"github.com/mineiros-io/terramate/hcl"

	. "github.com/mineiros-io/terramate/test/hclutils"
	. "github.com/mineiros-io/terramate/test/hclutils/info"
	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
)

func TestHCLParserGenerateBlocksRange(t *testing.T) {
	tcases := []testcase{
		{
			name: "multiple files",
			input: []cfgfile{
				{
					filename: "genfile.tm",
					body: GenerateFile(
						Labels("file.txt"),
						Str("content", "terramate is awesome"),
					).String(),
				},
				{
					filename: "genhcl.tm",
					body: GenerateHCL(
						Labels("file.hcl"),
						Content(),
					).String(),
				},
			},
			want: want{
				config: hcl.Config{
					Generate: hcl.GenerateConfig{
						Files: []hcl.GenFileBlock{
							{
								Label: "file.txt",
								Range: Range(
									"genfile.tm",
									Start(1, 1, 0),
									End(3, 2, 63),
								),
							},
						},
						HCLs: []hcl.GenHCLBlock{
							{
								Label: "file.hcl",
								Range: Range(
									"genhcl.tm",
									Start(1, 1, 0),
									End(4, 2, 43),
								),
							},
						},
					},
				},
			},
		},
		{
			name: "multiple blocks",
			input: []cfgfile{
				{
					filename: "generates.tm",
					body: Doc(
						GenerateFile(
							Labels("file.txt"),
							Str("content", "terramate is awesome"),
						),
						GenerateHCL(
							Labels("file.hcl"),
							Content(),
						),
					).String(),
				},
			},
			want: want{
				config: hcl.Config{
					Generate: hcl.GenerateConfig{
						Files: []hcl.GenFileBlock{
							{
								Label: "file.txt",
								Range: Range(
									"generates.tm",
									Start(1, 1, 0),
									End(3, 2, 63),
								),
							},
						},
						HCLs: []hcl.GenHCLBlock{
							{
								Label: "file.hcl",
								Range: Range(
									"generates.tm",
									Start(4, 1, 64),
									End(7, 2, 107),
								),
							},
						},
					},
				},
			},
		},
	}

	for _, tcase := range tcases {
		testParser(t, tcase)
	}
}
