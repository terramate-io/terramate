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
//     - try(terramate.non_existant, 1) -> try(1) -> 1
//     - try(globals.non_existant, locals.b, "default") -> try(locals.b, "default")
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
		var evaluated hclwrite.Tokens
		var err error
		var skip int

		next := tokens[pos+1]
		if next.Type == hclsyntax.TokenIdent && string(next.Bytes) == "for" {
			evaluated, skip, err = evalForExpr(
				tokens[pos:],
				ctx,
				hclsyntax.TokenOBrace,
				hclsyntax.TokenCBrace,
			)
		} else {
			evaluated, skip, err = evalObject(tokens[pos:], ctx)
		}

		if err != nil {
			return nil, 0, err
		}

		pos += skip
		out = append(out, evaluated...)

	case hclsyntax.TokenOBrack:
		var evaluated hclwrite.Tokens
		var err error
		var skip int

		next := tokens[pos+1]
		if next.Type == hclsyntax.TokenIdent && string(next.Bytes) == "for" {
			evaluated, skip, err = evalForExpr(
				tokens[pos:],
				ctx,
				hclsyntax.TokenOBrack,
				hclsyntax.TokenCBrack,
			)
		} else {
			evaluated, skip, err = evalList(tokens[pos:], ctx)
		}

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
	if len(tokens) < 2 {
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

func evalForExpr(
	tokens hclwrite.Tokens,
	eval *Context,
	matchOpenType hclsyntax.TokenType,
	matchCloseType hclsyntax.TokenType,
) (hclwrite.Tokens, int, error) {
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

	// { for <ident>,<ident>,...
	pos++
	for pos < len(tokens) && string(tokens[pos].Bytes) != "in" {
		tok = tokens[pos]
		if tok.Type != hclsyntax.TokenIdent {
			return nil, 0, errorf("invalid `for` expression: found %s", tok.Type)
		}

		out = append(out, tok)

		pos++
		tok = tokens[pos]
		if tok.Type == hclsyntax.TokenComma {
			out = append(out, tok)
			pos++
		}
	}

	tok = tokens[pos]
	if tok.Type != hclsyntax.TokenIdent {
		panic(errorf("found the `in` bytes of %s type instead of IDENT", tok.Type))
	}

	out = append(out, tok)

	// consume everything and give errors in case of terramate variables being
	// used in the `for`.
	pos++
	matchingCollectionTokens := 1
	for pos < len(tokens) && matchingCollectionTokens > 0 {
		tok = tokens[pos]
		if tok.Type == matchOpenType {
			matchingCollectionTokens++
		} else if tok.Type == matchCloseType {
			matchingCollectionTokens--
		}
		v, found := parseVariable(tokens[pos:])
		if found {
			if v.isTerramate {
				return nil, 0, fmt.Errorf(
					"`for` expression does not support terramate variables (globals, terramate)",
				)
			}

			out = append(out, v.alltokens()...)
			pos += v.size()
		} else {
			pos++
			out = append(out, tok)
		}
	}

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
				v, _ := parseVariable(tokens[pos:])
				skip = v.size()
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

func parseVariable(tokens hclwrite.Tokens) (v variable, found bool) {
	if len(tokens) < 3 {
		// a variable has at least the format: a.b
		return variable{}, false
	}

	if tokens[0].Type != hclsyntax.TokenIdent {
		return variable{}, false
	}

	pos := 1
	wantDot := true
	for pos < len(tokens) {
		tok := tokens[pos]

		if wantDot {
			if tok.Type != hclsyntax.TokenDot {
				break
			}
		} else {
			if tok.Type != hclsyntax.TokenIdent {
				break
			}
		}

		pos++
		wantDot = !wantDot
	}

	if pos < 3 {
		// found <IDENT> <DOT> so not a variable...
		return variable{}, false
	}

	v.name = tokens[:pos]
	nsvar := string(v.name[0].Bytes)
	v.isTerramate = nsvar == "global" || nsvar == "terramate"

	if pos < len(tokens) && tokens[pos].Type == hclsyntax.TokenOBrack {
		v.index = parseIndexing(tokens[pos:])

		pos += 1 + len(v.index) // "[" + tokens

		if tokens[pos].Type != hclsyntax.TokenCBrack {
			panic(sprintf("malformed variable: %s", tokens[pos].Type))
		}
	}

	return v, true
}

func parseIndexing(tokens hclwrite.Tokens) hclwrite.Tokens {
	if tokens[0].Type != hclsyntax.TokenOBrack {
		panic("not an indexing")
	}

	pos := 1

	v, found := parseVariable(tokens[pos:])
	if found {
		return v.alltokens()
	}

	pos += v.size()

	count := 0
	for ; pos < len(tokens) && tokens[pos].Type != hclsyntax.TokenCBrack; pos++ {
		// here be dragons
		// in other words: we don't validate the index expression, as it's going
		// to be evaluated by hashicorp library anyway (if global/terramate) or
		// ignored otherwise. Let's trust that hcl.Parse() catches all the issues.
		count++
	}

	if tokens[count+1].Type != hclsyntax.TokenCBrack {
		panic("unexpected")
	}

	count++

	return tokens[1:count]
}

func evalVar(tokens hclwrite.Tokens, ctx *Context) (hclwrite.Tokens, int, error) {
	out := hclwrite.Tokens{}

	v, found := parseVariable(tokens)
	if !found {
		panic("expect a variable")
	}

	if !v.isTerramate {
		out = append(out, v.alltokens()...)
		return out, v.size(), nil
	}

	var expr []byte

	for _, et := range v.alltokens() {
		expr = append(expr, et.Bytes...)
	}

	e, diags := hclsyntax.ParseExpression(expr, "gen.hcl", hcl.Pos{})
	if diags.HasErrors() {
		return nil, 0, errorf("failed to parse expr: %v", diags.Error())
	}

	val, err := ctx.Eval(e)
	if err != nil {
		// return the skip size for the try().
		return nil, v.size(), err
	}

	newtoks := hclwrite.TokensForValue(val)
	out = append(out, newtoks...)
	return out, v.size(), nil
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
				out = append(out, tokenInterpBegin())
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

// variable is a low-level representation of a variable in terms of tokens.
type variable struct {
	name  hclwrite.Tokens
	index hclwrite.Tokens

	isTerramate bool
}

func (v variable) alltokens() hclwrite.Tokens {
	tokens := v.name
	if len(v.index) > 0 {
		tokens = append(tokens, tokenOBrack())
		tokens = append(tokens, v.index...)
		tokens = append(tokens, tokenCBrack())
	}
	return tokens
}

func (v variable) size() int {
	sz := len(v.name)
	if len(v.index) > 0 {
		sz += len(v.index) + 2 // `[` <tokens> `]`
	}
	return sz
}

var sprintf = fmt.Sprintf
var errorf = fmt.Errorf
