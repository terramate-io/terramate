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

package lex

import (
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

// StringLiteralTokens creates all tokens to represent the given string as an hcl
// literal string.
func StringLiteralTokens(literal string) hclwrite.Tokens {
	return hclwrite.Tokens{
		TokenOQuote(),
		TokenQuotedLit(literal),
		TokenCQuote(),
	}
}

// TokenQuotedLit creates a new quoted literal token
func TokenQuotedLit(name string) *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenQuotedLit,
		Bytes: []byte(`"` + name + `"`),
	}
}

// TokenOQuote creates a new open quote " token.
func TokenOQuote() *hclwrite.Token {
	return &hclwrite.Token{
		Type: hclsyntax.TokenOQuote,
	}
}

// TokenCQuote creates a new close quote " token.
func TokenCQuote() *hclwrite.Token {
	return &hclwrite.Token{
		Type: hclsyntax.TokenCQuote,
	}
}
