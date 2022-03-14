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

package eval

import (
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

func Partial(fname string, tokens hclwrite.Tokens, ctx *Context) (hclwrite.Tokens, error) {
	pos := 0
	out := hclwrite.Tokens{}
	for pos < len(tokens) {
		evaluated, skip, err := evalExpr(false, fname, tokens[pos:], ctx)
		if err != nil {
			return nil, err
		}

		if skip == 0 {
			panic("serious bug")
		}

		pos += skip
		out = append(out, evaluated...)
	}

	if pos < len(tokens) {
		panic(fmt.Errorf("failed to evaluate all tokens: %d != %d", pos, len(tokens)))
	}

	return out, nil
}

func evalIdent(fname string, tokens hclwrite.Tokens, ctx *Context) (hclwrite.Tokens, int, error) {
	out := hclwrite.Tokens{}

	if !isCanEvaluateIdent(tokens) {
		return nil, 0, fmt.Errorf("malformed code")
	}

	pos := 0
	tok := tokens[pos]
	if tok.Type != hclsyntax.TokenIdent {
		return nil, 0, fmt.Errorf("unexpected token '%s' (%s)", tok.Bytes, tok.Type)
	}

	next := tokens[pos+1]
	switch next.Type {
	case hclsyntax.TokenDot:
		evaluated, skip, err := evalOneVar(fname, tokens[pos:], ctx)
		if err != nil {
			return nil, skip, err
		}

		pos += skip
		out = append(out, evaluated...)
	case hclsyntax.TokenOParen:
		evaluated, skip, err := evalFuncall(fname, tokens[pos:], ctx)
		if err != nil {
			return nil, skip, err
		}

		pos += skip
		out = append(out, evaluated...)
	default:
		panic("ident cannot be evaluated")
	}

	return out, pos, nil
}

func isCanEvaluateIdent(tokens hclwrite.Tokens) bool {
	if len(tokens) < 3 {
		return false
	}

	if tokens[0].Type != hclsyntax.TokenIdent {
		panic("bug: expects an IDENT at pos 0")
	}

	next := tokens[1]
	return next.Type == hclsyntax.TokenDot || next.Type == hclsyntax.TokenOParen
}

func evalList(fname string, tokens hclwrite.Tokens, ctx *Context) (hclwrite.Tokens, int, error) {
	pos := 0
	tok := tokens[pos]

	if tok.Type != hclsyntax.TokenOBrack {
		panic("bug")
	}

	out := hclwrite.Tokens{tok}

	pos++
	for pos < len(tokens) && tokens[pos].Type != hclsyntax.TokenCBrack {
		if tokens[pos].Type == hclsyntax.TokenNewline {
			out = append(out, tokens[pos])
			pos++
			continue
		}

		evaluated, skip, err := evalExpr(false, fname, tokens[pos:], ctx)
		if err != nil {
			return nil, 0, err
		}

		pos += skip
		out = append(out, evaluated...)

		tok := tokens[pos]
		if tok.Type == hclsyntax.TokenComma {
			pos++
			out = append(out, tok)
		}
	}

	if pos >= len(tokens) {
		return nil, 0, fmt.Errorf("malformed list")
	}

	tok = tokens[pos]
	if tok.Type != hclsyntax.TokenCBrack {
		return nil, 0, fmt.Errorf("malformed list, unexpected %s", tok.Bytes)
	}

	pos++
	out = append(out, tok)

	return out, pos, nil
}

func evalObject(fname string, tokens hclwrite.Tokens, ctx *Context) (hclwrite.Tokens, int, error) {
	pos := 0
	tok := tokens[pos]

	if tok.Type != hclsyntax.TokenOBrace {
		panic("bug")
	}

	out := hclwrite.Tokens{tok}

	pos++
	for pos < len(tokens) && tokens[pos].Type != hclsyntax.TokenCBrace {
		if tokens[pos].Type == hclsyntax.TokenNewline {
			out = append(out, tokens[pos])
			pos++
			continue
		}

		lfs, skip, err := evalKeyExpr(fname, tokens[pos:], ctx)
		if err != nil {
			return nil, 0, err
		}

		pos += skip
		out = append(out, lfs...)

		for tokens[pos].Type == hclsyntax.TokenNewline {
			out = append(out, tokens[pos])
			pos++
		}

		tok = tokens[pos]
		if tok.Type != hclsyntax.TokenEqual && tok.Type != hclsyntax.TokenColon {
			return nil, 0, fmt.Errorf("unexpected token '%s' (%s)", tok.Bytes, tok.Type)
		}

		out = append(out, tok)
		pos++

		rfs, skip, err := evalKeyExpr(fname, tokens[pos:], ctx)
		if err != nil {
			return nil, 0, err
		}

		pos += skip
		out = append(out, rfs...)

		tok = tokens[pos]
		if tok.Type == hclsyntax.TokenComma {
			out = append(out, tok)
			pos++
		}
	}

	if pos >= len(tokens) {
		return nil, 0, fmt.Errorf("malformed object")
	}

	tok = tokens[pos]
	if tok.Type != hclsyntax.TokenCBrace {
		return nil, 0, fmt.Errorf("malformed object, unexpected %s", tok.Bytes)
	}

	pos++
	out = append(out, tok)

	return out, pos, nil
}

func evalKeyExpr(fname string, tokens hclwrite.Tokens, ctx *Context) (hclwrite.Tokens, int, error) {
	return evalExpr(true, fname, tokens, ctx)
}

func evalExpr(iskey bool, fname string, tokens hclwrite.Tokens, ctx *Context) (hclwrite.Tokens, int, error) {
	if len(tokens) == 0 {
		return tokens, 0, nil
	}

	pos := 0

	out := hclwrite.Tokens{}

	tok := tokens[pos]
	switch tok.Type {
	case hclsyntax.TokenEOF:
		pos++
		if len(tokens) != pos {
			panic(fmt.Sprintf("got EOF at middle: %d < %d", pos, len(tokens)))
		}
	case hclsyntax.TokenOQuote:
		evaluated, skip, err := evalString(fname, tokens[pos:], ctx)
		if err != nil {
			return nil, 0, err
		}

		out = append(out, evaluated...)
		pos += skip
	case hclsyntax.TokenIdent:
		switch string(tok.Bytes) {
		case "true", "false", "null":
			out = append(out, tok)
			pos++
		default:
			if isCanEvaluateIdent(tokens[pos:]) {
				evaluated, skip, err := evalIdent(fname, tokens[pos:], ctx)
				if err != nil {
					return nil, 0, err
				}

				pos += skip
				out = append(out, evaluated...)
			} else if iskey {
				out = append(out, tok)
				pos++
			} else {
				return nil, 0, fmt.Errorf("expression cannot evaluate IDENT at this point")
			}
		}
	case hclsyntax.TokenOBrace:
		evaluated, skip, err := evalObject(fname, tokens[pos:], ctx)
		if err != nil {
			return nil, 0, err
		}

		pos += skip
		out = append(out, evaluated...)

	case hclsyntax.TokenOBrack:
		evaluated, skip, err := evalList(fname, tokens[pos:], ctx)
		if err != nil {
			return nil, 0, err
		}

		pos += skip
		out = append(out, evaluated...)
	case hclsyntax.TokenNumberLit:
		out = append(out, tok)
		pos++
	default:
		panic(fmt.Errorf("not implemented: %s (%s)", tok.Bytes, tok.Type))
	}

	if pos == 0 {
		panic("bug: no advance in the position")
	}

	return out, pos, nil
}

func evalFuncall(fname string, tokens hclwrite.Tokens, ctx *Context) (hclwrite.Tokens, int, error) {
	if len(tokens) < 3 {
		return nil, 0, fmt.Errorf("not a funcall")
	}

	pos := 0
	tok := tokens[pos]

	if tok.Type != hclsyntax.TokenIdent {
		return nil, 0, fmt.Errorf("malformed funcall, not start with IDENT")
	}

	functok := tok
	funcname := string(tok.Bytes)

	pos++
	if tokens[pos].Type != hclsyntax.TokenOParen {
		return nil, 0, fmt.Errorf("not a funcall")
	}

	oparenTok := tokens[pos]

	args := []hclwrite.Tokens{}

	pos++
	for pos < len(tokens) && tokens[pos].Type != hclsyntax.TokenCParen {
		evaluated, skip, err := evalExpr(false, fname, tokens[pos:], ctx)
		if err != nil {
			if funcname == "try" {
				// TODO(i4k): create a type/sentinel/whatever.
				if !strings.Contains(err.Error(), "evaluating expression") {
					return nil, 0, err
				}
				skip = countVarParts(tokens[pos:])
			} else {
				return nil, 0, err
			}
		}

		pos += skip
		if len(evaluated) > 0 {
			args = append(args, evaluated)
		}

		if tokens[pos].Type == hclsyntax.TokenComma {
			// commas will be emited later.
			pos++
		}
	}

	if pos >= len(tokens) {
		return nil, 0, fmt.Errorf("malformed funcall")
	}

	cloparenTok := tokens[pos]
	if cloparenTok.Type != hclsyntax.TokenCParen {
		panic("bug: funcall not closed")
	}

	pos++

	out := hclwrite.Tokens{}

	if funcname == "try" && len(args) == 1 {
		out = append(out, args[0]...)
	} else {
		out = append(out, functok, oparenTok)
		for i, arg := range args {
			out = append(out, arg...)
			if i != len(args)-1 {
				out = append(out, &hclwrite.Token{
					Type:  hclsyntax.TokenComma,
					Bytes: []byte(","),
				})
			}
		}
		out = append(out, cloparenTok)
	}

	return out, pos, nil
}

func interpTokenStart() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenTemplateInterp,
		Bytes: []byte("${"),
	}
}

func countVarParts(tokens hclwrite.Tokens) int {
	count := 0
	for i := 0; i < len(tokens); i++ {
		if tokens[i].Type != hclsyntax.TokenIdent && tokens[i].Type != hclsyntax.TokenDot {
			return count
		}
		count++
	}

	return count
}

func evalOneVar(fname string, tokens hclwrite.Tokens, ctx *Context) (hclwrite.Tokens, int, error) {
	out := hclwrite.Tokens{}

	if len(tokens) < 3 {
		return nil, 0, fmt.Errorf("expected a.b but got %d tokens", len(tokens))
	}

	varLen := countVarParts(tokens)

	if string(tokens[0].Bytes) != "global" &&
		string(tokens[0].Bytes) != "terramate" {
		out = append(out, tokens[:varLen]...)
		return out, varLen, nil
	}

	var expr []byte

	for _, et := range tokens[:varLen] {
		expr = append(expr, et.Bytes...)
	}

	e, diags := hclsyntax.ParseExpression(expr, fname, hcl.Pos{})
	if diags.HasErrors() {
		return nil, 0, fmt.Errorf("failed to parse expr: %v", diags.Error())
	}

	val, err := ctx.Eval(e)
	if err != nil {
		// return the skip size for the try().
		return nil, varLen, err
	}

	newtoks := hclwrite.TokensForValue(val)
	out = append(out, newtoks...)
	return out, varLen, nil
}

func evalString(fname string, tokens hclwrite.Tokens, ctx *Context) (hclwrite.Tokens, int, error) {
	tok := tokens[0]
	if tok.Type != hclsyntax.TokenOQuote {
		return nil, 0, fmt.Errorf("bug: not a quoted string")
	}

	out := hclwrite.Tokens{}
	out = append(out, tok)

	pos := 1
	didEval := false
	hasPrevQuoteLit := false

	for pos < len(tokens) && tokens[pos].Type != hclsyntax.TokenCQuote {
		tok := tokens[pos]
		switch tok.Type {
		case hclsyntax.TokenQuotedLit:
			out = append(out, tok)
			hasPrevQuoteLit = true
			pos++
		case hclsyntax.TokenTemplateSeqEnd:
			if !didEval {
				out = append(out, tok)
			}
			didEval = false
			pos++
		case hclsyntax.TokenTemplateInterp:
			pos++

			evaluated, skipArgs, err := evalIdent(fname, tokens[pos:], ctx)
			if err != nil {
				return nil, 0, err
			}

			if string(evaluated[0].Bytes) != string(tokens[pos].Bytes) {
				didEval = true

				if hasPrevQuoteLit {
					str := out[len(out)-1]
					switch evaluated[0].Type {
					case hclsyntax.TokenOQuote:
						if evaluated[1].Type != hclsyntax.TokenQuotedLit {
							panic(fmt.Errorf("unexpectedd type %s", evaluated[1].Type))
						}

						str.Bytes = append(str.Bytes, evaluated[1].Bytes...)
					default:
						panic(fmt.Sprintf("%s (%s)", evaluated[0].Bytes, evaluated[0].Type))
					}
				} else {
					out = append(out, evaluated...)
				}
			} else {
				out = append(out, interpTokenStart())
				out = append(out, evaluated...)
			}

			pos += skipArgs

		default:
			panic(fmt.Errorf("unexpected token here %s (%s)", tok.Bytes, tok.Type))
		}
	}

	if pos >= len(tokens) {
		return nil, 0, fmt.Errorf("malformed quoted string %d %d", len(tokens), pos)
	}

	tok = tokens[pos]
	if tok.Type != hclsyntax.TokenCQuote {
		return nil, 0, fmt.Errorf("malformed quoted string, expected '\"' (close quote)")
	}

	out = append(out, tok)
	pos++

	return out, pos, nil
}
