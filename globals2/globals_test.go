// Copyright 2023 Mineiros GmbH
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

package globals2_test

import (
	"path/filepath"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	hhclwrite "github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/globals2"
	"github.com/mineiros-io/terramate/hcl/ast"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/stdlib"
	"github.com/mineiros-io/terramate/test"
	errtest "github.com/mineiros-io/terramate/test/errors"
	"github.com/mineiros-io/terramate/test/hclwrite"
	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
	"github.com/mineiros-io/terramate/test/sandbox"
	"github.com/rs/zerolog"
)

type (
	hclconfig struct {
		path     string
		filename string
		add      *hclwrite.Block
	}
	testcase struct {
		name    string
		layout  []string
		configs []hclconfig
		expr    string
		evalDir string
		want    string
		wantErr error
	}
)

func TestGlobals3(t *testing.T) {
	for _, tc := range []testcase{
		{
			name:    "no globals",
			expr:    "1",
			evalDir: "/",
			want:    "1",
		},
		{
			name:    "no globals but with funcalls",
			expr:    `tm_upper("terramate is fun")`,
			evalDir: "/",
			want:    `"TERRAMATE IS FUN"`,
		},
		{
			name: "empty labeled globals creates objects",
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Labels("obj"),
					),
				},
			},
			expr:    `global.obj`,
			evalDir: "/",
			want:    `{}`,
		},
		{
			name: "empty labeled globals creates objects - multiple labels",
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Labels("obj", "a", "b", "c"),
					),
				},
			},
			expr:    `global.obj`,
			evalDir: "/",
			want: `{
				a = {
					b = {
						c = {}
					}
				}
			}`,
		},
		{
			name:   "single stack with a single global",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Str("a", "string"),
					),
				},
			},
			evalDir: "/stack",
			expr:    `global.a`,
			want:    `"string"`,
		},
		{
			name:   "extending global in the same scope",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						Globals(
							Labels("obj"),
							Str("a", "string"),
						),
						Globals(
							Labels("obj"),
							Str("b", "string"),
						),
					),
				},
			},
			evalDir: "/stack",
			expr:    `global.obj`,
			want: `{
				a = "string"
				b = "string"
			}`,
		},
		{
			name:   "extended globals outside the target ref range are ignored",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						Globals(
							Labels("obj"),
							Str("a", "string"),
						),
						Globals(
							Labels("obj"),
							Expr("fail", "crash()"),
						),
					),
				},
			},
			evalDir: "/stack",
			expr:    `global.obj.a`,
			want:    `"string"`,
		},
		{
			name:   "not referenced globals are not evaluated",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Str("a", "value"),
						Expr("fail_if_evaluated", `crash()`),
					),
				},
			},
			evalDir: "/stack",
			expr:    `global.a`,
			want:    `"value"`,
		},
		{
			name:   "single stack with target global depending on same scoped global",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Expr("a", `global.b`),
						Expr("b", `tm_upper("terramate is fun")`),
					),
				},
			},
			evalDir: "/stack",
			expr:    `global.a`,
			want:    `"TERRAMATE IS FUN"`,
		},
		{
			name:   "extending parent globals",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Labels("obj"),
						Expr("a", `"test"`),
						Expr("b", `tm_upper("test")`),
					),
				},
				{
					path: "/stack",
					add: Globals(
						Labels("obj", "c"),
						Expr("a", `"c.a"`),
					),
				},
			},
			evalDir: "/stack",
			expr:    `global.obj`,
			want: `{
				a = "test"
				b = "TEST"
				c = {
					a = "c.a"
				}
			}`,
		},
		{
			name:   "extending same key from parent globals",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Labels("obj"),
						Expr("a", `"test"`),
						Expr("b", `tm_upper("test")`),
					),
				},
				{
					path: "/stack",
					add: Globals(
						Labels("obj"),
						Expr("a", `"stackval"`),
					),
				},
			},
			evalDir: "/stack",
			expr:    `global.obj.a`,
			want:    `"stackval"`,
		},
		{
			name:   "extending same key from parent globals but targeting root object",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Labels("obj"),
						Expr("a", `"test"`),
						Expr("b", `tm_upper("test")`),
					),
				},
				{
					path: "/stack",
					add: Globals(
						Labels("obj"),
						Expr("a", `"stackval"`),
					),
				},
			},
			evalDir: "/stack",
			expr:    `global.obj`,
			want: `{
				a = "stackval"
				b = "TEST"
			}`,
		},
		{
			name:   "extending parent globals but referencing child defined part -- should not descend into parent",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Labels("obj"),
						Expr("a", `crash()`),
					),
				},
				{
					path: "/stack",
					add: Globals(
						Labels("obj", "c"),
						Expr("a", `"c.a"`),
					),
				},
			},
			evalDir: "/stack",
			expr:    `global.obj.c`,
			want: `{
				a = "c.a"
			}`,
		},
		{
			name:   "single stack with target global depending on multiple same scoped globals",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Expr("cfg", `{
							name = global.name
							domain = global.domain
						}`),
						Expr("name", `tm_upper("terramate")`),
						Str("domain", `terramate.io`),
					),
				},
			},
			evalDir: "/stack",
			expr:    `global.cfg`,
			want: `{
				domain = "terramate.io"
				name = "TERRAMATE"
			}`,
		},
		{
			name:   "globals with 2 dependency hops",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Expr("cfg", `{
							name = global.indirect
						}`),
						Expr("indirect", `tm_upper(global.name)`),
						Str("name", `terramate`),
					),
				},
			},
			evalDir: "/stack",
			expr:    `global.cfg`,
			want: `{
				name = "TERRAMATE"
			}`,
		},
		{
			name:   "globals with 5 dependency hops",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Expr("obj", `{
							val = global.a1
						}`),
						Expr("a1", `tm_upper(global.a2)`),
						Expr("a2", `tm_lower(global.a3)`),
						Expr("a3", `tm_upper(global.a4)`),
						Expr("a4", `"a4"`),
					),
				},
			},
			evalDir: "/stack",
			expr:    `global.obj`,
			want: `{
				val = "A4"
			}`,
		},
		{
			name:   "single stack with global dependency from parent",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Str("name", "terramate"),
					),
				},
				{
					path: "/stack",
					add: Globals(
						Expr("a", `global.name`),
					),
				},
			},
			evalDir: "/stack",
			expr:    `global.a`,
			want:    `"terramate"`,
		},
		{
			name:   "global dependency from parent with multiple hops",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Expr("a1", `global.a2`),
						Expr("a2", `"a2"`),
					),
				},
				{
					path: "/stack",
					add: Globals(
						Expr("a", `global.a1`),
					),
				},
			},
			evalDir: "/stack",
			expr:    `global.a`,
			want:    `"a2"`,
		},
		{
			name:   "global dependency from parent with lazy evaluation",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Expr("a1", `global.a2`),
						Expr("a2", `global.stackval`),
					),
				},
				{
					path: "/stack",
					add: Globals(
						Expr("a", `global.a1`),
						Str("stackval", "value from stack"),
					),
				},
			},
			evalDir: "/stack",
			expr:    `global.a`,
			want:    `"value from stack"`,
		},
		{
			name:   "globals with cycles in the same scope",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Expr("obj", `{
							val = global.a1
						}`),
						Expr("a1", `tm_upper(global.a2)`),
						Expr("a2", `tm_lower(global.a3)`),
						Expr("a3", `tm_upper(global.a4)`),
						Expr("a4", `global.a1`),
					),
				},
			},
			evalDir: "/stack",
			expr:    `global.obj`,
			wantErr: errors.E(globals2.ErrCycle),
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tc.layout)
			for _, globalBlock := range tc.configs {
				path := filepath.Join(s.RootDir(), globalBlock.path)
				filename := config.DefaultFilename
				if globalBlock.filename != "" {
					filename = globalBlock.filename
				}
				test.AppendFile(t, path, filename, globalBlock.add.String())
			}

			cfg, err := config.LoadRoot(s.RootDir())
			if err != nil {
				errtest.Assert(t, err, tc.wantErr)
				return
			}

			tree, ok := cfg.Lookup(project.NewPath(tc.evalDir))
			if !ok {
				t.Fatalf("evalDir %s not found", tc.evalDir)
			}

			expr, diags := hclsyntax.ParseExpression([]byte(tc.expr), "test.hcl", hcl.InitialPos)
			if diags.HasErrors() {
				t.Fatal(diags.Error())
			}

			ctx := eval.NewContext(stdlib.Functions(tree.HostDir()))
			g := globals2.New(ctx, tree)
			val, err := g.Eval(expr)
			errtest.Assert(t, err, tc.wantErr)

			if tc.wantErr != nil {
				return
			}

			assert.EqualStrings(t, string(hhclwrite.Format([]byte(tc.want))),
				string(hhclwrite.Format(ast.TokensForValue(val).Bytes())))
		})
	}
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
