package hcl_test

import (
	"testing"

	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/hcl/eval"
	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
)

func TestHCLParserAssert(t *testing.T) {
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
							Assertion: newExpr(t, "1 == 1"),
							Message:   newExpr(t, "global.message"),
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
							Assertion: newExpr(t, "1 == 1"),
							Message:   newExpr(t, "global.message"),
							Warning:   newExpr(t, "true"),
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
							Assertion: newExpr(t, "1 == 1"),
							Message:   newExpr(t, "global.message"),
						},
						{
							Origin:    "assert.tm",
							Assertion: newExpr(t, "666 == 1"),
							Message:   newExpr(t, "global.another"),
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
							Assertion: newExpr(t, "1 == 1"),
							Message:   newExpr(t, "global.message"),
						},
						{
							Origin:    "assert2.tm",
							Assertion: newExpr(t, "666 == 1"),
							Message:   newExpr(t, "global.another"),
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

func newExpr(t *testing.T, expr string) hhcl.Expression {
	t.Helper()

	res, err := eval.ParseExpressionBytes([]byte(expr))
	assert.NoError(t, err)
	return res
}
