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

	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/test"
	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
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
							Origin:    "assert.tm",
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
							Origin:    "assert.tm",
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
							Origin:    "assert.tm",
							Assertion: expr(t, "1 == 1"),
							Message:   expr(t, "global.message"),
						},
						{
							Origin:    "assert.tm",
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
							Origin:    "assert.tm",
							Assertion: expr(t, "1 == 1"),
							Message:   expr(t, "global.message"),
						},
						{
							Origin:    "assert2.tm",
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
						mkrange("assert.tm", start(1, 1, 0), end(3, 2, 37)),
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
						mkrange("assert.tm", start(1, 1, 0), end(3, 2, 29)),
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
						mkrange("assert.tm", start(4, 3, 61), end(4, 14, 72)),
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
						mkrange("assert.tm", start(1, 8, 7), end(1, 14, 13)),
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
						mkrange("assert.tm", start(4, 3, 61), end(4, 9, 67)),
					),
				},
			},
		},
	}

	for _, tcase := range tcases {
		testParser(t, tcase)
	}
}
