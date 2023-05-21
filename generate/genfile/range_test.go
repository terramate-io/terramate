// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package genfile_test

import (
	"testing"

	. "github.com/terramate-io/terramate/test/hclutils"
	. "github.com/terramate-io/terramate/test/hclutils/info"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
)

func TestGenerateFileRange(t *testing.T) {
	t.Parallel()

	tcases := []testcase{
		{
			name:  "multiple blocks",
			stack: "/stack",
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
			name:  "multiple files",
			stack: "/stack",
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
