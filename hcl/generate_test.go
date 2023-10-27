// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package hcl_test

import (
	"testing"

	"github.com/terramate-io/terramate/hcl"

	. "github.com/terramate-io/terramate/test/hclutils"
	. "github.com/terramate-io/terramate/test/hclutils/info"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
)

func TestHCLParserGenerateBlocksRange(t *testing.T) {
	t.Parallel()
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
