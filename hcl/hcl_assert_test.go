// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package hcl_test

import (
	"testing"

	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/test"
	. "github.com/terramate-io/terramate/test/hclutils"
	. "github.com/terramate-io/terramate/test/hclutils/info"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
)

func TestHCLParserAssert(t *testing.T) {
	expr := test.NewExpr
	tcases := []testcase{
		{
			name: "single assert",
			input: []cfgfile{
				{
					filename: "assert.tm",
					body: Assert(
						Expr("assertion", "1 == 1"),
						Expr("message", "global.message"),
					).String(),
				},
			},
			want: want{
				config: hcl.Config{
					Asserts: []hcl.AssertConfig{
						{
							Assertion: expr(t, "1 == 1"),
							Message:   expr(t, "global.message"),
						},
					},
				},
			},
		},
		{
			name: "assert with warning",
			input: []cfgfile{
				{
					filename: "assert.tm",
					body: Assert(
						Expr("warning", "true"),
						Expr("assertion", "1 == 1"),
						Expr("message", "global.message"),
					).String(),
				},
			},
			want: want{
				config: hcl.Config{
					Asserts: []hcl.AssertConfig{
						{
							Assertion: expr(t, "1 == 1"),
							Message:   expr(t, "global.message"),
							Warning:   expr(t, "true"),
						},
					},
				},
			},
		},
		{
			name: "multiple asserts on same file",
			input: []cfgfile{
				{
					filename: "assert.tm",
					body: Doc(
						Assert(
							Expr("assertion", "1 == 1"),
							Expr("message", "global.message"),
						),
						Assert(
							Expr("assertion", "666 == 1"),
							Expr("message", "global.another"),
						),
					).String(),
				},
			},
			want: want{
				config: hcl.Config{
					Asserts: []hcl.AssertConfig{
						{
							Assertion: expr(t, "1 == 1"),
							Message:   expr(t, "global.message"),
						},
						{
							Assertion: expr(t, "666 == 1"),
							Message:   expr(t, "global.another"),
						},
					},
				},
			},
		},
		{
			name: "multiple asserts on multiple files",
			input: []cfgfile{
				{
					filename: "assert.tm",
					body: Assert(
						Expr("assertion", "1 == 1"),
						Expr("message", "global.message"),
					).String(),
				},
				{
					filename: "assert2.tm",
					body: Assert(
						Expr("assertion", "666 == 1"),
						Expr("message", "global.another"),
					).String(),
				},
			},
			want: want{
				config: hcl.Config{
					Asserts: []hcl.AssertConfig{
						{
							Assertion: expr(t, "1 == 1"),
							Message:   expr(t, "global.message"),
						},
						{
							Assertion: expr(t, "666 == 1"),
							Message:   expr(t, "global.another"),
						},
					},
				},
			},
		},
		{
			name: "multiple asserts on multiple files have proper range",
			input: []cfgfile{
				{
					filename: "assert.tm",
					body: Assert(
						Expr("assertion", "1 == 1"),
						Expr("message", "global.message"),
					).String(),
				},
				{
					filename: "assert2.tm",
					body: Assert(
						Expr("assertion", "666 == 1"),
						Expr("message", "global.another"),
					).String(),
				},
			},
			want: want{
				config: hcl.Config{
					Asserts: []hcl.AssertConfig{
						{
							Range: Range(
								"assert.tm",
								Start(1, 1, 0),
								End(4, 2, 60),
							),
							Assertion: expr(t, "1 == 1"),
							Message:   expr(t, "global.message"),
						},
						{
							Range: Range(
								"assert2.tm",
								Start(1, 1, 0),
								End(4, 2, 62),
							),
							Assertion: expr(t, "666 == 1"),
							Message:   expr(t, "global.another"),
						},
					},
				},
			},
		},
		{
			name: "assertion is obligatory",
			input: []cfgfile{
				{
					filename: "assert.tm",
					body: Assert(
						Expr("message", "global.message"),
					).String(),
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("assert.tm", Start(1, 1, 0), End(3, 2, 37)),
					),
				},
			},
		},
		{
			name: "message is obligatory",
			input: []cfgfile{
				{
					filename: "assert.tm",
					body: Assert(
						Expr("assertion", "true"),
					).String(),
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("assert.tm", Start(1, 1, 0), End(3, 2, 29)),
					),
				},
			},
		},
		{
			name: "sub block fails",
			input: []cfgfile{
				{
					filename: "assert.tm",
					body: Assert(
						Expr("assertion", "1 == 1"),
						Expr("message", "global.message"),
						Block("something"),
					).String(),
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("assert.tm", Start(4, 3, 61), End(4, 12, 70)),
					),
				},
			},
		},
		{
			name: "label fails",
			input: []cfgfile{
				{
					filename: "assert.tm",
					body: Assert(
						Labels("ohno"),
						Expr("assertion", "1 == 1"),
						Expr("message", "global.message"),
					).String(),
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("assert.tm", Start(1, 8, 7), End(1, 14, 13)),
					),
				},
			},
		},
		{
			name: "unknown attribute fails",
			input: []cfgfile{
				{
					filename: "assert.tm",
					body: Assert(
						Expr("assertion", "1 == 1"),
						Expr("message", "global.message"),
						Expr("oopsie", "unknown"),
					).String(),
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("assert.tm", Start(4, 3, 61), End(4, 9, 67)),
					),
				},
			},
		},
	}

	for _, tcase := range tcases {
		testParser(t, tcase)
	}
}

func TestHCLParserAssertInsideGenerate(t *testing.T) {
	expr := test.NewExpr
	tcases := []testcase{
		{
			name: "single assert",
			input: []cfgfile{
				{
					filename: "assert_genfile.tm",
					body: GenerateFile(
						Labels("file.txt"),
						Str("content", "terramate is awesome"),
						Assert(
							Expr("assertion", "1 == 1"),
							Expr("message", "global.message"),
						),
					).String(),
				},
				{
					filename: "assert_genhcl.tm",
					body: GenerateHCL(
						Labels("file.hcl"),
						Content(),
						Assert(
							Expr("assertion", "1 == 1"),
							Expr("message", "global.message"),
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
								Asserts: []hcl.AssertConfig{
									{
										Assertion: expr(t, "1 == 1"),
										Message:   expr(t, "global.message"),
									},
								},
							},
						},
						HCLs: []hcl.GenHCLBlock{
							{
								Label: "file.hcl",
								Asserts: []hcl.AssertConfig{
									{
										Assertion: expr(t, "1 == 1"),
										Message:   expr(t, "global.message"),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "single assert with warning",
			input: []cfgfile{
				{
					filename: "assert_genfile.tm",
					body: GenerateFile(
						Labels("file.txt"),
						Str("content", "terramate is awesome"),
						Assert(
							Expr("assertion", "1 == 1"),
							Expr("message", "global.message"),
							Bool("warning", true),
						),
					).String(),
				},
				{
					filename: "assert_genhcl.tm",
					body: GenerateHCL(
						Labels("file.hcl"),
						Content(),
						Assert(
							Expr("assertion", "1 == 1"),
							Expr("message", "global.message"),
							Bool("warning", true),
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
								Asserts: []hcl.AssertConfig{
									{
										Assertion: expr(t, "1 == 1"),
										Message:   expr(t, "global.message"),
										Warning:   expr(t, "true"),
									},
								},
							},
						},
						HCLs: []hcl.GenHCLBlock{
							{
								Label: "file.hcl",
								Asserts: []hcl.AssertConfig{
									{
										Assertion: expr(t, "1 == 1"),
										Message:   expr(t, "global.message"),
										Warning:   expr(t, "true"),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "multiple asserts blocks",
			input: []cfgfile{
				{
					filename: "assert_genfile.tm",
					body: GenerateFile(
						Labels("file.txt"),
						Str("content", "terramate is awesome"),
						Assert(
							Expr("assertion", "1 == 1"),
							Expr("message", "global.message"),
						),
						Assert(
							Expr("assertion", "1 == 666"),
							Expr("message", "global.file"),
						),
					).String(),
				},
				{
					filename: "assert_genhcl.tm",
					body: GenerateHCL(
						Labels("file.hcl"),
						Content(),
						Assert(
							Expr("assertion", "1 == 1"),
							Expr("message", "global.message"),
						),
						Assert(
							Expr("assertion", "2 == 666"),
							Expr("message", "global.hcl"),
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
								Asserts: []hcl.AssertConfig{
									{
										Assertion: expr(t, "1 == 1"),
										Message:   expr(t, "global.message"),
									},
									{
										Assertion: expr(t, "1 == 666"),
										Message:   expr(t, "global.file"),
									},
								},
							},
						},
						HCLs: []hcl.GenHCLBlock{
							{
								Label: "file.hcl",
								Asserts: []hcl.AssertConfig{
									{
										Assertion: expr(t, "1 == 1"),
										Message:   expr(t, "global.message"),
									},
									{
										Assertion: expr(t, "2 == 666"),
										Message:   expr(t, "global.hcl"),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "multiple asserts blocks with range",
			input: []cfgfile{
				{
					filename: "assert_genfile.tm",
					body: GenerateFile(
						Labels("file.txt"),
						Str("content", "terramate is awesome"),
						Assert(
							Expr("assertion", "1 == 1"),
							Expr("message", "global.message"),
						),
						Assert(
							Expr("assertion", "1 == 666"),
							Expr("message", "global.file"),
						),
					).String(),
				},
				{
					filename: "assert_genhcl.tm",
					body: GenerateHCL(
						Labels("file.hcl"),
						Content(),
						Assert(
							Expr("assertion", "1 == 1"),
							Expr("message", "global.message"),
						),
						Assert(
							Expr("assertion", "2 == 666"),
							Expr("message", "global.hcl"),
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
								Asserts: []hcl.AssertConfig{
									{
										Range: Range(
											"assert_genfile.tm",
											Start(3, 3, 64),
											End(6, 4, 130),
										),
										Assertion: expr(t, "1 == 1"),
										Message:   expr(t, "global.message"),
									},
									{
										Range: Range(
											"assert_genfile.tm",
											Start(7, 3, 133),
											End(10, 4, 198),
										),
										Assertion: expr(t, "1 == 666"),
										Message:   expr(t, "global.file"),
									},
								},
							},
						},
						HCLs: []hcl.GenHCLBlock{
							{
								Label: "file.hcl",
								Asserts: []hcl.AssertConfig{
									{
										Range: Range(
											"assert_genhcl.tm",
											Start(4, 3, 44),
											End(7, 4, 110),
										),
										Assertion: expr(t, "1 == 1"),
										Message:   expr(t, "global.message"),
									},
									{
										Range: Range(
											"assert_genhcl.tm",
											Start(8, 3, 113),
											End(11, 4, 177),
										),
										Assertion: expr(t, "2 == 666"),
										Message:   expr(t, "global.hcl"),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "assertion is obligatory",
			input: []cfgfile{
				{
					filename: "assert_genfile.tm",
					body: GenerateFile(
						Labels("file.txt"),
						Str("content", "terramate is awesome"),
						Assert(
							Expr("message", "global.message"),
						),
					).String(),
				},
				{
					filename: "assert_genhcl.tm",
					body: GenerateHCL(
						Labels("file.hcl"),
						Content(),
						Assert(
							Expr("message", "global.message"),
						),
					).String(),
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("assert_genfile.tm", Start(3, 3, 64), End(5, 4, 105)),
					),
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("assert_genhcl.tm", Start(4, 3, 44), End(6, 4, 85)),
					),
				},
			},
		},
		{
			name: "message is obligatory",
			input: []cfgfile{
				{
					filename: "assert_genfile.tm",
					body: GenerateFile(
						Labels("file.txt"),
						Str("content", "terramate is awesome"),
						Assert(
							Expr("assertion", "global.data"),
						),
					).String(),
				},
				{
					filename: "assert_genhcl.tm",
					body: GenerateHCL(
						Labels("file.hcl"),
						Content(),
						Assert(
							Expr("assertion", "global.data"),
						),
					).String(),
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("assert_genfile.tm", Start(3, 3, 64), End(5, 4, 104)),
					),
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("assert_genhcl.tm", Start(4, 3, 44), End(6, 4, 84)),
					),
				},
			},
		},
		{
			name: "sub block fails",
			input: []cfgfile{
				{
					filename: "assert_genfile.tm",
					body: GenerateFile(
						Labels("file.txt"),
						Str("content", "terramate is awesome"),
						Assert(
							Expr("assertion", "global.data"),
							Expr("message", "global.msg"),
							Assert(
								Expr("assertion", "global.data"),
								Expr("message", "global.msg"),
							),
						),
					).String(),
				},
				{
					filename: "assert_genhcl.tm",
					body: GenerateHCL(
						Labels("file.hcl"),
						Content(),
						Assert(
							Expr("assertion", "global.data"),
							Expr("message", "global.msg"),
							Assert(
								Expr("assertion", "global.data"),
								Expr("message", "global.msg"),
							),
						),
					).String(),
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("assert_genfile.tm", Start(6, 5, 132), End(6, 11, 138)),
					),
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("assert_genhcl.tm", Start(7, 5, 112), End(7, 11, 118)),
					),
				},
			},
		},
		{
			name: "label fails",
			input: []cfgfile{
				{
					filename: "assert_genfile.tm",
					body: GenerateFile(
						Labels("file.txt"),
						Str("content", "terramate is awesome"),
						Assert(
							Labels("ohno"),
							Expr("assertion", "global.data"),
							Expr("message", "global.msg"),
						),
					).String(),
				},
				{
					filename: "assert_genhcl.tm",
					body: GenerateHCL(
						Labels("file.hcl"),
						Content(),
						Assert(
							Labels("ohno"),
							Expr("assertion", "global.data"),
							Expr("message", "global.msg"),
						),
					).String(),
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("assert_genfile.tm", Start(3, 10, 71), End(3, 16, 77)),
					),
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("assert_genhcl.tm", Start(4, 10, 51), End(4, 16, 57)),
					),
				},
			},
		},
		{
			name: "unknown attribute fails",
			input: []cfgfile{
				{
					filename: "assert_genfile.tm",
					body: GenerateFile(
						Labels("file.txt"),
						Str("content", "terramate is awesome"),
						Assert(
							Expr("assertion", "global.data"),
							Expr("message", "global.msg"),
							Expr("was", "global.msg"),
						),
					).String(),
				},
				{
					filename: "assert_genhcl.tm",
					body: GenerateHCL(
						Labels("file.hcl"),
						Content(),
						Assert(
							Expr("assertion", "global.data"),
							Expr("message", "global.msg"),
							Expr("was", "global.msg"),
						),
					).String(),
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("assert_genfile.tm", Start(6, 5, 132), End(6, 8, 135)),
					),
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("assert_genhcl.tm", Start(7, 5, 112), End(7, 8, 115)),
					),
				},
			},
		},
	}

	for _, tcase := range tcases {
		testParser(t, tcase)
	}
}
