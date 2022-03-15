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

// Partial evaluates only the terramate variable expressions from the list of
// tokens, leaving all the rest as-is. It returns a modified list of tokens with
// no reference to terramate namespaced variables (globals and terramate).
// Caveats:
//   - In the case of the `try` function, the terramate variables get removed if
//     non existent.
//   - The try() function gets completed removed and replaced by the default
//     value if only it is left as an argument.
func Partial(fname string, tokens hclwrite.Tokens, ctx *Context) (hclwrite.Tokens, error) {
	pos := 0
	out := hclwrite.Tokens{}
	for pos < len(tokens) {
		evaluated, skip, err := evalExpr(false, tokens[pos:], ctx)
		if err != nil {
			return nil, err
		}

		if skip == 0 {
			panic("eval.Partial: this should not have happened: please open a bug ticket.")
		}

		pos += skip
		out = append(out, evaluated...)
	}

	if pos < len(tokens) {
		panic("eval.Partial: this should not have happened: please open a bug ticket")
	}

	return out, nil
}

func evalExpr(iskey bool, tokens hclwrite.Tokens, ctx *Context) (hclwrite.Tokens, int, error) {
	out := hclwrite.Tokens{}

	pos := 0
	tok := tokens[pos]
	switch tok.Type {
	case hclsyntax.TokenEOF:
		pos++
		if len(tokens) != pos {
			panic(sprintf("got EOF in the middle: %d < %d", pos, len(tokens)))
		}
	case hclsyntax.TokenOQuote:
		evaluated, skip, err := evalString(tokens[pos:], ctx)
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
				evaluated, skip, err := evalIdent(tokens[pos:], ctx)
				if err != nil {
					return nil, 0, err
				}

				pos += skip
				out = append(out, evaluated...)
			} else if iskey {
				out = append(out, tok)
				pos++
			} else {
				return nil, 0, errorf(
					"expression cannot evaluate IDENT (%s) at this point",
					tok.Bytes,
				)
			}
		}
	case hclsyntax.TokenOBrace:
		next := tokens[pos+1]
		if next.Type == hclsyntax.TokenIdent && string(next.Bytes) == "for" {
			evaluated, skip, err := evalForExpr(tokens[pos:], ctx)
			if err != nil {
				return nil, 0, err
			}

			pos += skip
			out = append(out, evaluated...)
		} else {
			evaluated, skip, err := evalObject(tokens[pos:], ctx)
			if err != nil {
				return nil, 0, err
			}

			pos += skip
			out = append(out, evaluated...)
		}
	case hclsyntax.TokenOBrack:
		evaluated, skip, err := evalList(tokens[pos:], ctx)
		if err != nil {
			return nil, 0, err
		}

		pos += skip
		out = append(out, evaluated...)
	case hclsyntax.TokenNumberLit:
		out = append(out, tok)
		pos++
	default:
		panic(errorf("not implemented: %s (%s)", tok.Bytes, tok.Type))
	}

	if pos == 0 {
		panic("bug: no advance in the position")
	}

	return out, pos, nil
}

func evalIdent(tokens hclwrite.Tokens, ctx *Context) (hclwrite.Tokens, int, error) {
	out := hclwrite.Tokens{}

	if !isCanEvaluateIdent(tokens) {
		return nil, 0, errorf("malformed code")
	}

	pos := 0
	tok := tokens[pos]
	if tok.Type != hclsyntax.TokenIdent {
		return nil, 0, errorf("evalIdent: unexpected token '%s' (%s)", tok.Bytes, tok.Type)
	}

	next := tokens[pos+1]
	switch next.Type {
	case hclsyntax.TokenDot:
		evaluated, skip, err := evalVar(tokens[pos:], ctx)
		if err != nil {
			return nil, skip, err
		}

		pos += skip
		out = append(out, evaluated...)
	case hclsyntax.TokenOParen:
		evaluated, skip, err := evalFuncall(tokens[pos:], ctx)
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

func evalList(tokens hclwrite.Tokens, ctx *Context) (hclwrite.Tokens, int, error) {
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

		evaluated, skip, err := evalExpr(false, tokens[pos:], ctx)
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
		return nil, 0, errorf("malformed list")
	}

	tok = tokens[pos]
	if tok.Type != hclsyntax.TokenCBrack {
		return nil, 0, errorf("malformed list, unexpected %s", tok.Bytes)
	}

	pos++
	out = append(out, tok)

	return out, pos, nil
}

func evalObject(tokens hclwrite.Tokens, ctx *Context) (hclwrite.Tokens, int, error) {
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

		lfs, skip, err := evalKeyExpr(tokens[pos:], ctx)
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
			return nil, 0, errorf("evalObject: unexpected token '%s' (%s)", tok.Bytes, tok.Type)
		}

		out = append(out, tok)
		pos++

		rfs, skip, err := evalKeyExpr(tokens[pos:], ctx)
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
		return nil, 0, errorf("malformed object")
	}

	tok = tokens[pos]
	if tok.Type != hclsyntax.TokenCBrace {
		return nil, 0, errorf("malformed object, unexpected %s", tok.Bytes)
	}

	pos++
	out = append(out, tok)

	return out, pos, nil
}

func evalKeyExpr(tokens hclwrite.Tokens, ctx *Context) (hclwrite.Tokens, int, error) {
	return evalExpr(true, tokens, ctx)
}

func evalForExpr(tokens hclwrite.Tokens, ctx *Context) (hclwrite.Tokens, int, error) {
	// {
	pos := 0
	tok := tokens[pos]
	if tok.Type != hclsyntax.TokenOBrace {
		panic(sprintf("evalForExpr: malformed `for` expression: %s", tok.Bytes))
	}

	out := hclwrite.Tokens{tok}

	// { for
	pos++
	tok = tokens[pos]
	if tok.Type != hclsyntax.TokenIdent || string(tok.Bytes) != "for" {
		panic(sprintf("evalForExpr: malformed `for` expression: %s", tok.Bytes))
	}

	out = append(out, tok)

	// { for <ident>
	pos++
	tok = tokens[pos]
	if tok.Type != hclsyntax.TokenIdent {
		return nil, 0, errorf("invalid for expression")
	}

	out = append(out, tok)

	// { for <ident> in
	pos++
	tok = tokens[pos]
	if tok.Type != hclsyntax.TokenIdent && string(tok.Bytes) != "in" {
		return nil, 0, errorf("invalid `for` expression: expected `in`")
	}

	out = append(out, tok)

	// { for <ident> in <expression>
	pos++
	evaluated, skip, err := evalExpr(false, tokens[pos:], ctx)
	if err != nil {
		return nil, 0, err
	}

	pos += skip
	out = append(out, evaluated...)

	// { for <ident> in <expression> :
	tok = tokens[pos]
	if tok.Type != hclsyntax.TokenColon {
		return nil, 0, errorf("invalid `for` expression: expected `:`")
	}

	out = append(out, tok)

	// { for <ident> in <expression> : <expression>
	pos++
	evaluated, skip, err = evalExpr(true, tokens[pos:], ctx)
	if err != nil {
		return nil, 0, err
	}

	pos += skip
	out = append(out, evaluated...)

	// { for <ident> in <expression> : <expression> =>
	tok = tokens[pos]
	if tok.Type != hclsyntax.TokenFatArrow {
		return nil, 0, errorf("evalForExpr: unexpected token `%s`, expected `=>`", tok.Bytes)
	}

	out = append(out, tok)

	// { for <ident> in <expression> : <expression> => <expression>
	pos++
	evaluated, skip, err = evalExpr(true, tokens[pos:], ctx)
	if err != nil {
		return nil, 0, err
	}

	pos += skip
	out = append(out, evaluated...)

	// { for <ident> in <expression> : <expression> => <expression> }
	tok = tokens[pos]
	if tok.Type != hclsyntax.TokenCBrace {
		return nil, 0, errorf("evalForExpr: unexpected token `%s`, expected `}`", tok.Bytes)
	}

	out = append(out, tok)
	pos++

	return out, pos, nil
}

func evalFuncall(tokens hclwrite.Tokens, ctx *Context) (hclwrite.Tokens, int, error) {
	if len(tokens) < 3 {
		return nil, 0, errorf("not a funcall")
	}

	pos := 0
	tok := tokens[pos]

	if tok.Type != hclsyntax.TokenIdent {
		return nil, 0, errorf("malformed funcall, not start with IDENT")
	}

	functok := tok
	funcname := string(tok.Bytes)

	pos++
	if tokens[pos].Type != hclsyntax.TokenOParen {
		return nil, 0, errorf("not a funcall")
	}

	oparenTok := tokens[pos]

	args := []hclwrite.Tokens{}

	pos++
	for pos < len(tokens) && tokens[pos].Type != hclsyntax.TokenCParen {
		evaluated, skip, err := evalExpr(false, tokens[pos:], ctx)
		if err != nil {
			if funcname == "try" {
				// TODO(i4k): create a type/sentinel/whatever.
				if !strings.Contains(err.Error(), "evaluating expression") {
					return nil, 0, err
				}
				skip, _ = varInfo(false, tokens[pos:])
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
		return nil, 0, errorf("malformed funcall")
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

func varInfo(isIndexing bool, tokens hclwrite.Tokens) (count int, index hclwrite.Tokens) {
	count = 0
loop:
	for i := 0; i < len(tokens); i++ {
		switch tokens[i].Type {
		case hclsyntax.TokenNumberLit:
			if !isIndexing {
				break loop
			}
			count++
			return count, nil
		case hclsyntax.TokenOQuote, hclsyntax.TokenQuotedLit:
			if !isIndexing {
				break loop
			}

			count++
		case hclsyntax.TokenCQuote:
			count++
			return count, nil
		case hclsyntax.TokenIdent, hclsyntax.TokenDot:
			count++
		default:
			break loop
		}
	}

	if count < len(tokens) && tokens[count].Type == hclsyntax.TokenOBrack {
		count++
		indexSize, indexTokens := varInfo(true, tokens[count:])
		count += indexSize
		index = indexTokens

		if tokens[count].Type != hclsyntax.TokenCBrack {
			panic(sprintf("malformed variable: %s", tokens[count].Type))
		}

		count++
	}

	return count, index
}

func evalVar(tokens hclwrite.Tokens, ctx *Context) (hclwrite.Tokens, int, error) {
	out := hclwrite.Tokens{}

	if len(tokens) < 3 {
		return nil, 0, errorf("expected a.b but got %d tokens", len(tokens))
	}

	varLen, index := varInfo(false, tokens)

	if string(tokens[0].Bytes) != "global" &&
		string(tokens[0].Bytes) != "terramate" {
		out = append(out, tokens[:varLen]...)
		return out, varLen, nil
	}

	if len(index) > 0 && index[0].Type != hclsyntax.TokenNumberLit {
		return nil, 0, errorf("evalVar: indexing kind not implemented: %s", index[0].Type)
	}

	var expr []byte

	for _, et := range tokens[:varLen] {
		expr = append(expr, et.Bytes...)
	}

	e, diags := hclsyntax.ParseExpression(expr, "gen.hcl", hcl.Pos{})
	if diags.HasErrors() {
		return nil, 0, errorf("failed to parse expr: %v", diags.Error())
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

func evalString(tokens hclwrite.Tokens, ctx *Context) (hclwrite.Tokens, int, error) {
	tok := tokens[0]
	if tok.Type != hclsyntax.TokenOQuote {
		return nil, 0, errorf("bug: not a quoted string")
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

			evaluated, skipArgs, err := evalIdent(tokens[pos:], ctx)
			if err != nil {
				return nil, 0, err
			}

			// TODO(i4k): improve this.
			if string(evaluated[0].Bytes) != string(tokens[pos].Bytes) {
				didEval = true

				if hasPrevQuoteLit {
					str := out[len(out)-1]
					switch evaluated[0].Type {
					case hclsyntax.TokenOQuote:
						if evaluated[1].Type != hclsyntax.TokenQuotedLit {
							panic(errorf("unexpected type %s", evaluated[1].Type))
						}

						str.Bytes = append(str.Bytes, evaluated[1].Bytes...)
					case hclsyntax.TokenNumberLit, hclsyntax.TokenIdent: // numbers, true, false, null
						str.Bytes = append(str.Bytes, evaluated[0].Bytes...)
					default:
						panic(sprintf("interpolation kind not supported: %s (%s)",
							evaluated[0].Bytes, evaluated[0].Type))
					}
				} else {
					switch evaluated[0].Type {
					case hclsyntax.TokenOQuote, hclsyntax.TokenOBrack, hclsyntax.TokenOBrace:
						out = append(out, evaluated[1:len(evaluated)-1]...)
					case hclsyntax.TokenNumberLit, hclsyntax.TokenIdent: // numbers, true, false, null
						out = append(out, evaluated...)
					default:
						panic(sprintf("interpolation kind not supported: %s (%s)",
							evaluated[0].Bytes, evaluated[0].Type))
					}

				}
			} else {
				out = append(out, interpTokenStart())
				out = append(out, evaluated...)
			}

			pos += skipArgs

		default:
			panic(errorf("evalString: unexpected token %s (%s)", tok.Bytes, tok.Type))
		}
	}

	if pos >= len(tokens) {
		return nil, 0, errorf("malformed quoted string %d %d", len(tokens), pos)
	}

	tok = tokens[pos]
	if tok.Type != hclsyntax.TokenCQuote {
		return nil, 0, errorf("malformed quoted string, expected '\"' (close quote)")
	}

	out = append(out, tok)
	pos++

	return out, pos, nil
}

var sprintf = fmt.Sprintf
var errorf = fmt.Errorf
