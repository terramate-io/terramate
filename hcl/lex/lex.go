package lex

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/mineiros-io/terramate/errors"
)

// TokenIdent creates a new identifier token
func TokenIdent(name string) *hclwrite.Token {
	return &hclwrite.Token{}
}

// TokenOBrace creates a new { token.
func TokenOBrace() *hclwrite.Token {
	return &hclwrite.Token{}
}

// TokenEquals compare if two tokens have the same type and bytes.
func TokenEquals(token1, token2 *hclwrite.Token) bool {
	return false
}

// FindTokenSequence finds the first match of the given token sequence
// on the given tokens, returning the position of the last token of the
// matched sequence, so the next token (if any) would be what is after the
// given sequence. The contents of the tokens are also matched, not only their type.
func FindTokenSequence(tokens hclwrite.Tokens, seq ...*hclwrite.Token) (int, bool) {
	if len(seq) == 0 {
		return 0, false
	}
	matchHead := seq[0]

searchMatch:
	for i, token := range tokens {
		if !TokenEquals(token, matchHead) {
			continue
		}

		pendingMatches := seq[1:]
		position := i + len(pendingMatches)

		for _, token := range tokens[i+1:] {
			if len(pendingMatches) == 0 {
				break
			}
			if !TokenEquals(token, pendingMatches[0]) {
				continue searchMatch
			}
			pendingMatches = pendingMatches[1:]
		}

		if len(pendingMatches) == 0 {
			return position, true
		}
	}

	return 0, false
}

// Config performs lexical analysis on the given buffer, treating it as a
// whole HCL config file, and returns the resulting tokens.
//
// Only minimal validation is done during lexical analysis, so the returned
// error may include errors about lexical issues such as bad character
// encodings or unrecognized characters, but full parsing is required to
// detect _all_ syntax errors.
func Config(src []byte, filename string) (hclsyntax.Tokens, error) {
	tokens, diags := hclsyntax.LexConfig(src, filename, hcl.Pos{Byte: 0, Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, errors.E(diags, "lexing configuration")
	}
	return tokens, nil
}

// WriterTokens takes a sequence of tokens as produced by the hclsyntax
// package and transforms it into an equivalent sequence of tokens
// from hclwrite, which then can be properly saved back on a file.
//
// The resulting list contains the same number of tokens and uses the same
// indices as the input, allowing the two sets of tokens to be correlated by index.
func WriterTokens(nativeTokens hclsyntax.Tokens) hclwrite.Tokens {
	// This is mostly copied from a private function on hclwrite.
	tokBuf := make([]hclwrite.Token, len(nativeTokens))
	var lastByteOffset int
	for i, mainToken := range nativeTokens {
		bytes := make([]byte, len(mainToken.Bytes))
		copy(bytes, mainToken.Bytes)

		tokBuf[i] = hclwrite.Token{
			Type:  mainToken.Type,
			Bytes: bytes,
			// We assume here that spaces are always ASCII spaces, since
			// that's what the scanner also assumes, and thus the number
			// of bytes skipped is also the number of space characters.
			SpacesBefore: mainToken.Range.Start.Byte - lastByteOffset,
		}

		lastByteOffset = mainToken.Range.End.Byte
	}

	ret := make(hclwrite.Tokens, len(tokBuf))
	for i := range ret {
		ret[i] = &tokBuf[i]
	}
	return ret
}
