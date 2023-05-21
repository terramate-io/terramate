// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package genhcl_test

import (
	"testing"

	. "github.com/terramate-io/terramate/test/hclutils"
	. "github.com/terramate-io/terramate/test/hclutils/info"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
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
