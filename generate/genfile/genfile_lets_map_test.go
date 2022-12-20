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

	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/lets"
	maptest "github.com/mineiros-io/terramate/mapexpr/test"
	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
)

func TestGenFileLetsMapSchemaErrors(t *testing.T) {
	for _, maptc := range maptest.SchemaErrorTestcases() {
		tc := testcase{
			name:  "genfile with lets and " + maptc.Name,
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/file.tm",
					add: GenerateFile(
						Labels("test"),
						Lets(
							Str("name", "value"),
							maptc.Block,
						),
						Expr("content", "let.name"),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		}
		testGenfile(t, tc)
	}
}

func TestGenFileLetsMap(t *testing.T) {
	t.Parallel()
	for _, tc := range []testcase{
		{
			name:  "lets.map label conflicts with lets name",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/genfile.tm",
					add: GenerateFile(
						Labels("test"),
						Lets(
							Str("name", "value"),
							Map(
								Labels("name"),
								Expr("for_each", "[]"),
								Str("key", "a"),
								Str("value", "a"),
							),
						),
						Expr("content", "let.name"),
					),
				},
			},
			wantErr: errors.E(lets.ErrRedefined),
		},
		{
			name:  "lets with simple map block",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/genfile.tm",
					add: GenerateFile(
						Labels("test"),
						Lets(
							Map(
								Labels("var"),
								Expr("for_each", `["a", "b", "c"]`),
								Expr("key", "element.new"),
								Expr("value", "element.new"),
							),
						),
						Str("content", "${let.var.a}-${let.var.b}-${let.var.c}"),
					),
				},
			},
			want: []result{
				{
					name: "test",
					file: genFile{
						condition: true,
						body:      "a-b-c",
					},
				},
			},
		},
		{
			name:  "lets with map using iterator",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/genfile.tm",
					add: GenerateFile(
						Labels("test"),
						Lets(
							Map(
								Labels("var"),
								Expr("iterator", "el"),
								Expr("for_each", `["a", "b", "c"]`),
								Expr("key", "el.new"),
								Expr("value", "el.new"),
							),
						),
						Str("content", "${let.var.a}-${let.var.b}-${let.var.c}"),
					),
				},
			},
			want: []result{
				{
					name: "test",
					file: genFile{
						condition: true,
						body:      "a-b-c",
					},
				},
			},
		},
		{
			name:  "lets with map block with incorrect key",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/genfile.tm",
					add: GenerateFile(
						Labels("test"),
						Lets(
							Map(
								Labels("var"),
								Expr("for_each", `["a", "b", "c"]`),
								Expr("key", "something"), // keyword is not valid
								Str("value", "else"),
							),
						),
						Str("content", "${let.var.a}-${let.var.b}-${let.var.c}"),
					),
				},
			},
			wantErr: errors.E(lets.ErrEval),
		},
		{
			name:  "lets with map block with incorrect value",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/genfile.tm",
					add: GenerateFile(
						Labels("test"),
						Lets(
							Map(
								Labels("var"),
								Expr("for_each", `["a", "b", "c"]`),
								Expr("value", "something"), // keyword is not valid
								Str("key", "else"),
							),
						),
						Str("content", "${let.var.a}-${let.var.b}-${let.var.c}"),
					),
				},
			},
			wantErr: errors.E(lets.ErrEval),
		},
		{
			name:  "lets with simple map without using element",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/genfile.tm",
					add: GenerateFile(
						Labels("test"),
						Lets(
							Map(
								Labels("var"),
								Expr("for_each", `["a", "b", "c"]`),
								Str("key", "something"),
								Str("value", "else"),
							),
						),
						Expr("content", "let.var.something"),
					),
				},
			},
			want: []result{
				{
					name: "test",
					file: genFile{
						condition: true,
						body:      "else",
					},
				},
			},
		},
	} {
		testGenfile(t, tc)
	}
}
