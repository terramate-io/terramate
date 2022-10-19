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

	"github.com/mineiros-io/terramate/config"
	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
)

func TestGenerateHCLAssert(t *testing.T) {
	t.Parallel()

	t.Skip()

	tcases := []testcase{
		{
			name:  "multiple assert configs accessing metadata/globals/lets",
			stack: "/stack",
			configs: []hclconfig{
				{
					path:     "/stack",
					filename: "globals.tm",
					add: Globals(
						Str("a", "value"),
					),
				},
				{
					path:     "/stack",
					filename: "generate.tm",
					add: GenerateHCL(
						Labels("asserts.hcl"),
						Lets(
							Expr("a", "global.a"),
						),
						Assert(
							Expr("assertion", "let.a == global.a"),
							Str("message", "let.a != global.a"),
						),
						Assert(
							Expr("assertion", `terramate.stack.path == "/stack"`),
							Str("message", "wrong stack metadata"),
						),
						Content(
							Str("a", "lets.a"),
						),
					),
				},
			},
			want: []result{
				{
					name: "asserts.hcl",
					hcl: genHCL{
						origin:    "/stack/generate.tm",
						condition: true,
						body: Doc(
							Str("a", "value"),
						),
						asserts: []config.Assert{
							{
								Assertion: true,
								Message:   "let.a != global.a",
							},
							{
								Assertion: true,
								Message:   "wrong stack metadata",
							},
						},
					},
				},
			},
		},
	}

	for _, tcase := range tcases {
		tcase.run(t)
	}
}
