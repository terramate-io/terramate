// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build go1.18 && !windows

package eval_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/rs/zerolog"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

func FuzzPartialEval(f *testing.F) {
	seedCorpus := []string{
		"attr",
		"attr.value",
		"attr.*.value",
		"global.str",
		`"a ${global.str}"`,
		`"${global.obj}"`,
		`"${global.list} fail`,
		`"domain is ${tm_replace(global.str, "io", "com")}"`,
		`{}`,
		`{
			global.str = 1
			b = 2
		}`,
		`10`,
		`"test"`,
		`[1, 2, 3]`,
		`a()`,
		`föo("föo") + föo`,
		`${var.name}`,
		`{ for k in var.val : k => k }`,
		`[ for k in var.val : k => k ]`,
		`<<EOT
		${local.var}
EOT`,
	}

	for _, seed := range seedCorpus {
		f.Add(seed)
	}

	s := sandbox.New(f)
	root := s.Config()

	f.Fuzz(func(t *testing.T, str string) {
		// WHY? because HCL uses the big.Float library for numbers and then
		// fuzzer can generate huge number strings like 100E101000000 that will
		// hang the process and eat all the memory....
		const bigNumRegex = "[\\d]+[\\s]*[.]?[\\s]*[\\d]*[EepP]{1}[\\s]*[+-]?[\\s]*[\\d]+"
		hasBigNumbers, _ := regexp.MatchString(bigNumRegex, str)
		if hasBigNumbers {
			return
		}

		// the hcl library has a bug evaluating funcalls containing variations
		// of this ternary operation.
		if strings.Contains(strings.ReplaceAll(str, " ", ""), "!0,0?[]") {
			return
		}

		parsedExpr, diags := hclsyntax.ParseExpression([]byte(str), "fuzz.hcl", hcl.InitialPos)
		if diags.HasErrors() {
			return
		}

		//resolver := globals.NewResolver(root.Tree())

		globalsStmts := eval.Stmts{
			test.NewStmt(t, `global.str`, `"mineiros.io"`),
			test.NewStmt(t, `global.bool`, `true`),
			test.NewStmt(t, `global.list`, `[1, 2, 3]`),
		}

		globalsStmts = append(globalsStmts, test.NewStmtFrom(t, `global.obj`, `{
			a = "b"
			b = "c"
			c = "d"
		}`)...)

		ctx := eval.New(
			root.Tree().Dir(),
			runtime.NewResolver(root, nil),
			globals.NewResolver(
				root,
				globalsStmts...,
			),
		)

		gotExpr, err := ctx.PartialEval(parsedExpr)
		if err != nil {
			return
		}
		for _, v := range gotExpr.Variables() {
			exprBytes := ast.TokensForExpression(gotExpr).Bytes()
			if (v.RootName() == "global" || v.RootName() == "terramate") &&
				strings.Contains(string(exprBytes), v.RootName()+".") {
				t.Fatalf(
					"not all Terramate references replaced: input: %s, output: %s",
					str, exprBytes,
				)
			}
		}
	})
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
