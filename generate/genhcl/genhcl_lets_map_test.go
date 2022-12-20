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

	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/lets"
	maptest "github.com/mineiros-io/terramate/mapexpr/test"
	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
)

func TestGenHCLLetsMapSchemaErrors(t *testing.T) {
	for _, maptc := range maptest.SchemaErrorTestcases() {
		tc := testcase{
			name:  "genhcl with lets and " + maptc.Name,
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateFile(
						Labels("test.tf"),
						Lets(
							Str("name", "value"),
							maptc.Block,
						),
						Content(
							Expr("name", "let.name"),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		}
		tc.run(t)
	}
}

func TestGenHCLLetsMap(t *testing.T) {
	t.Parallel()

	for _, tc := range []testcase{
		{
			name:  "lets.map label conflicts with lets name",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("test.tf"),
						Lets(
							Str("name", "value"),
							Map(
								Labels("name"),
								Expr("for_each", "[]"),
								Str("key", "a"),
								Str("value", "a"),
							),
						),
						Content(
							Expr("name", "let.name"),
						),
					),
				},
			},
			wantErr: errors.E(lets.ErrRedefined),
		},
		{
			name:  "lets with map block",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("test.tf"),
						Lets(
							Map(
								Labels("var"),
								Expr("for_each", `["a", "b", "c"]`),
								Expr("key", "element.new"),
								Expr("value", "element.new"),
							),
						),
						Content(
							Str("content", "${let.var.a}-${let.var.b}-${let.var.c}"),
						),
					),
				},
			},
			want: []result{
				{
					name: "test.tf",
					hcl: genHCL{
						condition: true,
						body: Doc(
							Str("content", "a-b-c"),
						),
					},
				},
			},
		},
		{
			name:  "lets with map block using iterator",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("test.tf"),
						Lets(
							Map(
								Labels("var"),
								Expr("iterator", "el"),
								Expr("for_each", `["a", "b", "c"]`),
								Expr("key", "el.new"),
								Expr("value", "el.new"),
							),
						),
						Content(
							Str("content", "${let.var.a}-${let.var.b}-${let.var.c}"),
						),
					),
				},
			},
			want: []result{
				{
					name: "test.tf",
					hcl: genHCL{
						condition: true,
						body: Doc(
							Str("content", "a-b-c"),
						),
					},
				},
			},
		},
	} {
		tc.run(t)
	}
}
