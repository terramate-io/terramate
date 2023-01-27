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

//go:build go1.18 && linux

package ast_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/go-test/deep"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mineiros-io/terramate/hcl/ast"
	"github.com/rs/zerolog"
)

func FuzzTokensForExpression(f *testing.F) {
	seedCorpus := []string{
		"attr",
		"attr.value",
		//"attr.*.value",
		"global.str",
		`"a ${global.str}"`,
		`"${global.obj}"`,
		`"${global.list} fail`,
		`{}`,
		`{a=[]}`,
		`[{}]`,
		`[{a=666}]`,
		`[[]]`,
		`10`,
		`"test"`,
		`[1, 2, 3]`,
		`[[1], [2], [3]]`,
		`a()`,
		`föo("föo") + föo`,
		`${var.name}`,
		`{ for k in var.val : k => k }`,
		`[ for k in var.val : k => k ]`,
	}

	for _, seed := range seedCorpus {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, str string) {
		// WHY? because HCL uses the big.Float library for numbers and then
		// fuzzer can generate huge number strings like 100E101000000 that will
		// hang the process and eat all the memory....
		const bigNumRegex = "[\\d]+[\\s]*[.]?[\\s]*[\\d]*[EepP]{1}[\\s]*[+-]?[\\s]*[\\d]+"
		hasBigNumbers, _ := regexp.MatchString(bigNumRegex, str)
		if hasBigNumbers {
			return
		}

		if strings.Contains(str, "/*") || strings.Contains(str, "//") || strings.Contains(str, "#") {
			// comments are meaningless here as they are not present in the AST
			return
		}

		expr, diags := hclsyntax.ParseExpression([]byte(str), "fuzz.hcl", hcl.InitialPos)
		if diags.HasErrors() {
			return
		}
		gotTokens := ast.TokensForExpression(expr)
		gotExpr, diags := hclsyntax.ParseExpression(gotTokens.Bytes(), "fuzz.hcl", hcl.InitialPos)
		if diags.HasErrors() {
			t.Fatalf("error: %v -> input: %s || generated: %s", diags.Error(), str, string(gotTokens.Bytes()))
		}
		for _, problem := range deep.Equal(gotExpr, expr) {
			if !strings.Contains(problem, ".Start.") && !strings.Contains(problem, ".End.") {
				t.Fatalf("%s, (got %T) (want %s) for %s and %s", problem, gotExpr, ast.TokensForExpression(expr.(*hclsyntax.TemplateExpr).Parts[1]).Bytes(), gotTokens.Bytes(), str)
			}
		}
	})
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
