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

//go:build go1.18 && !windows

package eval_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mineiros-io/terramate/globals"
	"github.com/mineiros-io/terramate/hcl/ast"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/runtime"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/sandbox"
	"github.com/rs/zerolog"
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
			runtime.NewResolver(root, nil),
			globals.NewResolver(
				root.Tree(),
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
