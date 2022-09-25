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

package eval

import (
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/madlambda/spells/assert"
	"github.com/rs/zerolog"
	"github.com/zclconf/go-cty/cty"
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
		`10`,
		`"test"`,
		`[1, 2, 3]`,
		`a()`,
		`föo("föo") + föo`,
		`${var.name}`,
		`{ for k in var.val : k => k }`,
		`[ for k in var.val : k => k ]`,
	}

	for _, seed := range seedCorpus {
		f.Add(seed)
	}

	globals := map[string]cty.Value{
		"str":  cty.StringVal("mineiros.io"),
		"bool": cty.BoolVal(true),
		"list": cty.ListVal([]cty.Value{
			cty.NumberVal(big.NewFloat(1)),
			cty.NumberVal(big.NewFloat(2)),
			cty.NumberVal(big.NewFloat(3)),
		}),
		"obj": cty.ObjectVal(map[string]cty.Value{
			"a": cty.StringVal("b"),
			"b": cty.StringVal("c"),
			"c": cty.StringVal("d"),
		}),
	}

	terramate := map[string]cty.Value{
		"path": cty.StringVal("/my/project"),
		"name": cty.StringVal("happy stack"),
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

		// the hcl library has a bug evaluating funcalls containing variations
		// of this ternary operation.
		if strings.Contains(strings.ReplaceAll(str, " ", ""), "!0,0?[]") {
			return
		}

		const testattr = "attr"

		cfg := fmt.Sprintf("%s = %s", testattr, str)
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCL([]byte(cfg), "fuzz")
		if diags.HasErrors() {
			return
		}

		body := file.Body.(*hclsyntax.Body)
		attr := body.Attributes[testattr]
		parsedExpr := attr.Expr

		exprRange := parsedExpr.Range()
		exprBytes := cfg[exprRange.Start.Byte:exprRange.End.Byte]

		parsedTokens, diags := hclsyntax.LexExpression([]byte(exprBytes), "fuzz", hcl.Pos{})
		if diags.HasErrors() {
			return
		}

		ctx, err := NewContext("/")
		assert.NoError(t, err)
		ctx.SetNamespace("globals", globals)
		ctx.SetNamespace("terramate", terramate)

		want := toWriteTokens(parsedTokens)
		engine := newPartialEvalEngine(want, ctx)
		got, err := engine.Eval()

		if strings.Contains(cfg, "global") ||
			strings.Contains(cfg, "terramate") ||
			strings.Contains(cfg, "tm_") {
			// TODO(katcipis): Validate generated code properties when
			// substitution is in play.
			return
		}

		assert.NoError(t, err)

		// Since we dont fuzz substitution/evaluation the tokens should be the same
		assert.EqualInts(t, len(want), len(got), "got %s != want %s", tokensStr(got), tokensStr(want))

		for i, gotToken := range got {
			wantToken := want[i]
			if diff := cmp.Diff(*gotToken, *wantToken); diff != "" {
				t.Errorf("got: %v", *gotToken)
				t.Errorf("want: %v", *wantToken)
				t.Error("diff:")
				t.Fatal(diff)
			}
		}
	})
}

func tokensStr(t hclwrite.Tokens) string {
	tokensStrs := make([]string, len(t))
	for i, token := range t {
		tokensStrs[i] = fmt.Sprintf("{Type=%q Bytes=%s}", token.Type, token.Bytes)
	}
	return "[" + strings.Join(tokensStrs, ",") + "]"
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
