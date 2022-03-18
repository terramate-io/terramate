//go:build go1.18

package eval_test

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/hcl/eval"
)

func FuzzPartialEval(f *testing.F) {
	seedCorpus := []string{"attr"}

	for _, seed := range seedCorpus {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, expr string) {
		parsedTokens, diags := hclsyntax.LexExpression([]byte(expr), "fuzz", hcl.Pos{})
		if diags.HasErrors() {
			return
		}

		tokens, err := eval.Partial("fuzz", toWriteTokens(parsedTokens), eval.NewContext(""))
		if err != nil {
			t.Fatal(err)
		}

		// Since we dont fuzz substitution, the amount of tokens should be the same
		assert.EqualInts(t, len(tokens), len(parsedTokens))
	})
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
