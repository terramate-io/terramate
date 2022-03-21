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

package eval_test

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
	"github.com/mineiros-io/terramate/hcl/eval"
	tmhclwrite "github.com/mineiros-io/terramate/test/hclwrite"
)

func FuzzPartialEval(f *testing.F) {
	seedCorpus := []string{
		"attr",
		"attr.value",
	}

	for _, seed := range seedCorpus {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, str string) {
		// Here we fuzz that anything that the hclsyntax lib handle we should
		// also handle with no errors. We dont fuzz actual substitution
		// scenarios that would require a proper context with globals loaded.

		cfg := hcldoc(
			expr("attr", str),
		)

		cfgString := cfg.String()
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCL([]byte(cfgString), "fuzz")
		if diags.HasErrors() {
			return
		}

		fmt.Printf("config: %s\n", cfgString)

		body := file.Body.(*hclsyntax.Body)
		attr := body.Attributes["attr"]
		parsedExpr := attr.Expr

		exprRange := parsedExpr.Range()
		exprBytes := cfgString[exprRange.Start.Byte:exprRange.End.Byte]

		parsedTokens, diags := hclsyntax.LexExpression([]byte(exprBytes), "fuzz", hcl.Pos{})
		if diags.HasErrors() {
			return
		}

		want := toWriteTokens(parsedTokens)
		engine := eval.NewEngine(want, eval.NewContext(""))
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

func toWriteTokens(in hclsyntax.Tokens) hclwrite.Tokens {
	tokens := make([]*hclwrite.Token, len(in))
	for i, st := range in {
		tokens[i] = &hclwrite.Token{
			Type:  st.Type,
			Bytes: st.Bytes,
		}
	}
	return tokens
}

func hcldoc(builders ...tmhclwrite.BlockBuilder) *tmhclwrite.Block {
	return tmhclwrite.BuildHCL(builders...)
}

func generateHCL(builders ...tmhclwrite.BlockBuilder) *tmhclwrite.Block {
	return tmhclwrite.BuildBlock("generate_hcl", builders...)
}

func block(name string, builders ...tmhclwrite.BlockBuilder) *tmhclwrite.Block {
	return tmhclwrite.BuildBlock(name, builders...)
}

func terraform(builders ...tmhclwrite.BlockBuilder) *tmhclwrite.Block {
	return tmhclwrite.BuildBlock("terraform", builders...)
}

func globals(builders ...tmhclwrite.BlockBuilder) *tmhclwrite.Block {
	return tmhclwrite.BuildBlock("globals", builders...)
}

func content(builders ...tmhclwrite.BlockBuilder) *tmhclwrite.Block {
	return tmhclwrite.BuildBlock("content", builders...)
}

func labels(labels ...string) tmhclwrite.BlockBuilder {
	return tmhclwrite.Labels(labels...)
}

func expr(name string, expr string) tmhclwrite.BlockBuilder {
	return tmhclwrite.Expression(name, expr)
}

func str(name string, val string) tmhclwrite.BlockBuilder {
	return tmhclwrite.String(name, val)
}

func number(name string, val int64) tmhclwrite.BlockBuilder {
	return tmhclwrite.NumberInt(name, val)
}

func boolean(name string, val bool) tmhclwrite.BlockBuilder {
	return tmhclwrite.Boolean(name, val)
}
