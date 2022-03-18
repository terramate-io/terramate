//go:build go1.18

package eval_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/hcl/eval"
)

func FuzzPartialEval(f *testing.F) {
	seedCorpus := []string{
		"attr",
		"attr.value",
	}

	for _, seed := range seedCorpus {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, expr string) {
		// Here we fuzz that anything that the hclsyntax lib handle we should
		// also handle with no errors. We dont fuzz actual substitution
		// scenarios that would require a proper context with globals loaded.
		parsedTokens, diags := hclsyntax.LexExpression([]byte(expr), "fuzz", hcl.Pos{})
		if diags.HasErrors() {
			return
		}

		want := toWriteTokens(parsedTokens)
		got, err := eval.Partial("fuzz", want, eval.NewContext(""))
		if err != nil {
			t.Fatal(err)
		}

		// Since we dont fuzz substitution, the amount of tokens should be the same
		assert.EqualInts(t, len(got), len(want), "got %s != want %s", tokensStr(got), tokensStr(want))

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
		tokensStrs[i] = fmt.Sprintf("{Type=%q Bytes=%v}", token.Type, token.Bytes)
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
