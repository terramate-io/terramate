// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package genfile_test

import (
	"testing"

	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/eval"
	. "github.com/terramate-io/terramate/test/hclutils"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
)

func TestGenerateFileAssert(t *testing.T) {
	t.Parallel()

	tcases := []testcase{
		{
			name:  "multiple assert configs accessing metadata/globals/lets",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/globals.tm",
					add: Globals(
						Str("a", "value"),
					),
				},
				{
					path: "/stack/generate.tm",
					add: GenerateFile(
						Labels("asserts.hcl"),
						Lets(
							Expr("a", "global.a"),
						),
						Assert(
							Expr("assertion", "let.a == global.a"),
							Str("message", "let.a != global.a"),
						),
						Assert(
							Expr("assertion", `terramate.stack.path.absolute == "/stack"`),
							Str("message", "wrong stack metadata"),
							Bool("warning", true),
						),
						Expr("content", "let.a"),
					),
				},
			},
			want: []result{
				{
					name: "asserts.hcl",
					file: genFile{
						condition: true,
						body:      "value",
						asserts: []config.Assert{
							{
								Range:     Mkrange("/stack/generate.tm", Start(7, 17, 88), End(7, 34, 105)),
								Assertion: true,
								Message:   "let.a != global.a",
							},
							{
								Range:     Mkrange("/stack/generate.tm", Start(11, 17, 173), End(11, 58, 214)),
								Assertion: true,
								Message:   "wrong stack metadata",
								Warning:   true,
							},
						},
					},
				},
			},
		},
		{
			name:  "if one assertion fails generated code will be empty",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/generate.tm",
					add: GenerateFile(
						Labels("asserts.hcl"),
						Assert(
							Expr("assertion", "true"),
							Str("message", "always true"),
						),
						Assert(
							Expr("assertion", `true == false`),
							Str("message", "such wrong"),
						),
						Expr("content", "global.this.will.explode"),
					),
				},
			},
			want: []result{
				{
					name: "asserts.hcl",
					file: genFile{
						condition: true,
						body:      "",
						asserts: []config.Assert{
							{
								Range:     Mkrange("/stack/generate.tm", Start(4, 17, 58), End(4, 21, 62)),
								Assertion: true,
								Message:   "always true",
							},
							{
								Range:     Mkrange("/stack/generate.tm", Start(8, 17, 124), End(8, 30, 137)),
								Assertion: false,
								Message:   "such wrong",
							},
						},
					},
				},
			},
		},
		{
			name:  "if condition is false asserts are not evaluated",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/generate.tm",
					add: GenerateFile(
						Labels("asserts.hcl"),
						Bool("condition", false),
						Assert(
							Expr("assertion", `global.explosions`),
							Expr("message", "let.such.undefined"),
						),
						Expr("content", "global.this.will.explode"),
					),
				},
			},
			want: []result{
				{
					name: "asserts.hcl",
					file: genFile{
						condition: false,
						body:      "",
					},
				},
			},
		},
		{
			name:  "if one assertion fails with warning code is generated",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/generate.tm",
					add: GenerateFile(
						Labels("asserts.hcl"),
						Assert(
							Expr("assertion", "true"),
							Str("message", "always true"),
						),
						Assert(
							Expr("assertion", `true == false`),
							Str("message", "such wrong"),
							Bool("warning", true),
						),
						Str("content", "generating code is fun"),
					),
				},
			},
			want: []result{
				{
					name: "asserts.hcl",
					file: genFile{
						condition: true,
						body:      "generating code is fun",
						asserts: []config.Assert{
							{
								Range:     Mkrange("/stack/generate.tm", Start(4, 17, 58), End(4, 21, 62)),
								Assertion: true,
								Message:   "always true",
							},
							{
								Range:     Mkrange("/stack/generate.tm", Start(8, 17, 124), End(8, 30, 137)),
								Assertion: false,
								Message:   "such wrong",
								Warning:   true,
							},
						},
					},
				},
			},
		},
		{
			name:  "evaluation failure",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/fail.tm",
					add: GenerateFile(
						Labels("asserts.hcl"),
						Assert(
							Expr("assertion", "let.a == global.a"),
							Str("message", "let.a != global.a"),
						),
						Str("content", "data"),
					),
				},
			},
			wantErr: errors.L(errors.E(eval.ErrEval), errors.E(eval.ErrEval)),
		},
	}

	for _, tcase := range tcases {
		testGenfile(t, tcase)
	}
}
