// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build go1.18 && linux

package ast_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/rs/zerolog"
	"github.com/terramate-io/terramate/hcl/ast"
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
		ast.TokensForExpression(expr)
	})
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
