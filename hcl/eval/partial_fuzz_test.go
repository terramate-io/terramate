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

//go:build go1.18

package eval

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/madlambda/spells/assert"

	tmhclwrite "github.com/mineiros-io/terramate/test/hclwrite"
)

func FuzzPartialEval(f *testing.F) {
	seedCorpus := []string{
		"attr",
		"attr.value",
		"attr.*.value",
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

	f.Fuzz(func(t *testing.T, str string) {
		// Here we fuzz that anything that the hclsyntax lib handle we should
		// also handle with no errors. We dont fuzz actual substitution
		// scenarios that would require a proper context with globals loaded.
		const testattr = "attr"

		cfg := hcldoc(
			expr(testattr, str),
		)

		cfgString := cfg.String()
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCL([]byte(cfgString), "fuzz")
		if diags.HasErrors() {
			return
		}

		body := file.Body.(*hclsyntax.Body)
		attr := body.Attributes[testattr]
		parsedExpr := attr.Expr

		exprRange := parsedExpr.Range()
		exprBytes := cfgString[exprRange.Start.Byte:exprRange.End.Byte]

		parsedTokens, diags := hclsyntax.LexExpression([]byte(exprBytes), "fuzz", hcl.Pos{})
		if diags.HasErrors() {
			return
		}

		want := toWriteTokens(parsedTokens)
		engine := newPartialEngine(want, NewContext(""))
		got, err := engine.PartialEval()

		if strings.Contains(cfgString, "global.") || strings.Contains(cfgString, "terramate.") {
			// TODO(katcipis): Validate generated code properties when
			// substitution is in play.
			return
		}

		if err != nil {
			t.Fatal(err)
		}

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

func hcldoc(builders ...tmhclwrite.BlockBuilder) *tmhclwrite.Block {
	return tmhclwrite.BuildHCL(builders...)
}

func expr(name string, expr string) tmhclwrite.BlockBuilder {
	return tmhclwrite.Expression(name, expr)
}
