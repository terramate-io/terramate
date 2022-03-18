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
	"github.com/madlambda/spells/errutil"
)

const (
	ErrPartialEval         errutil.Error = "partial evaluation failed"
	ErrForExprDisallowEval errutil.Error = "`for` expression disallow globals/terramate variables"
	ErrInterpolationEval   errutil.Error = "interpolation failed"
)

/*

Here be dragons. Thou art forewarned


                                             _   __,----'~~~~~~~~~`-----.__
                                      .  .    `//====-              ____,-'~`
                      -.            \_|// .   /||\\  `~~~~`---.___./
                ______-==.       _-~o  `\/    |||  \\           _,'`
          __,--'   ,=='||\=_    ;_,_,/ _-'|-   |`\   \\        ,'
       _-'      ,='    | \\`.    '',/~7  /-   /  ||   `\.     /
     .'       ,'       |  \\  \_  "  /  /-   /   ||      \   /
    / _____  /         |     \\.`-_/  /|- _/   ,||       \ /
   ,-'     `-|--'~~`--_ \     `==-/  `| \'--===-'       _/`
             '         `-|      /|    )-'\~'      _,--"'
                         '-~^\_/ |    |   `\_   ,^             /\
                              /  \     \__   \/~               `\__
                          _,-' _/'\ ,-'~____-'`-/                 ``===\
                         ((->/'    \|||' `.     `\.  ,                _||
           ./                       \_     `\      `~---|__i__i__\--~'_/
          <_n_                     __-^-_    `)  \-.______________,-~'
           `B'\)                  ///,-'~`__--^-  |-------~~~~^'
           /^>                           ///,--~`-\
          `  `
*/

// Partial evaluates only the terramate variable expressions from the list of
// tokens, leaving all the rest as-is. It returns a modified list of tokens with
// no reference to terramate namespaced variables (globals and terramate) and
// functions (tm_ prefixed functions).
func Partial(fname string, tokens hclwrite.Tokens, ctx *Context) (hclwrite.Tokens, error) {
	pos := 0
	out := hclwrite.Tokens{}
	for pos < len(tokens) {
		evaluated, skip, err := evalExpr(false, tokens[pos:], ctx)
		if err != nil {
			return nil, errutil.Chain(ErrPartialEval, err)
		}

		if skip == 0 {
			panic("eval.Partial: this should not have happened: please open a bug ticket.")
		}

		pos += skip
		out = append(out, evaluated...)
	}

	return out, nil
}

func evalExpr(iskey bool, tokens hclwrite.Tokens, ctx *Context) (hclwrite.Tokens, int, error) {
	out := hclwrite.Tokens{}
	pos := 0
	tok := tokens[pos]

	// exprTerm
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
	case hclsyntax.TokenOBrace, hclsyntax.TokenOBrack:
		var evaluated hclwrite.Tokens
		var err error
		var skip int

		var closeToken hclsyntax.TokenType

		openToken := tok.Type
		if openToken == hclsyntax.TokenOBrace {
			closeToken = hclsyntax.TokenCBrace
		} else {
			closeToken = hclsyntax.TokenCBrack
		}

		next := tokens[pos+1]
		switch {
		case isForExpr(next):
			evaluated, skip, err = evalForExpr(tokens[pos:], ctx, openToken, closeToken)
		case openToken == hclsyntax.TokenOBrace:
			evaluated, skip, err = evalObject(tokens[pos:], ctx)
		case openToken == hclsyntax.TokenOBrack:
			evaluated, skip, err = evalList(tokens[pos:], ctx)
		default:
			panic("unexpected")
		}

		if err != nil {
			return nil, 0, err
		}

		pos += skip
		out = append(out, evaluated...)
	case hclsyntax.TokenNumberLit:
		out = append(out, tok)
		pos++
	}

	if pos == 0 {
		panic(sprintf("bug: no advance in the position: %s", tokens[pos].Type))
	}

	if pos >= len(tokens) {
		return out, pos, nil
	}

	// operation && conditional

	tok = tokens[pos]
	switch tok.Type {
	case hclsyntax.TokenEqualOp, hclsyntax.TokenNotEqual,
		hclsyntax.TokenLessThan, hclsyntax.TokenLessThanEq,
		hclsyntax.TokenGreaterThan, hclsyntax.TokenGreaterThanEq:
		out = append(out, tok)
		pos++

		evaluated, skip, err := evalExpr(false, tokens[pos:], ctx)
		if err != nil {
			return nil, 0, err
		}

		pos += skip
		out = append(out, evaluated...)
	case hclsyntax.TokenPlus, hclsyntax.TokenMinus,
		hclsyntax.TokenStar, hclsyntax.TokenSlash, hclsyntax.TokenPercent:
		out = append(out, tok)
		pos++
		evaluated, skip, err := evalExpr(false, tokens[pos:], ctx)
		if err != nil {
			return nil, 0, err
		}

		pos += skip
		out = append(out, evaluated...)
	case hclsyntax.TokenQuestion:
		out = append(out, tok)
		pos++

		evaluated, skip, err := evalExpr(false, tokens[pos:], ctx)
		if err != nil {
			return nil, 0, err
		}

		pos += skip
		out = append(out, evaluated...)

		if tokens[pos].Type != hclsyntax.TokenColon {
			return nil, 0, fmt.Errorf(
				"expected `:` but found a %s (%s)",
				tokens[pos].Bytes, tokens[pos].Type,
			)
		}

		out = append(out, tokens[pos])
		pos++

		evaluated, skip, err = evalExpr(false, tokens[pos:], ctx)
		if err != nil {
			return nil, 0, err
		}

		pos += skip
		out = append(out, evaluated...)
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

func isForExpr(tok *hclwrite.Token) bool {
	return tok.Type == hclsyntax.TokenIdent && string(tok.Bytes) == "for"
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
	// { | [
	pos := 0
	tok := tokens[pos]
	if tok.Type != matchOpenType {
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
				return nil, 0, errutil.Chain(
					ErrForExprDisallowEval,
					fmt.Errorf("evaluating expression: %s", v.alltokens().Bytes()),
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

func isTmFuncall(tok *hclwrite.Token) bool {
	return tok.Type == hclsyntax.TokenIdent &&
		strings.HasPrefix(string(tok.Bytes), "tm_")
}

func evalTmFuncall(tokens hclwrite.Tokens, ctx *Context) (hclwrite.Tokens, int, error) {
	if len(tokens) < 3 {
		return nil, 0, errorf("not a funcall")
	}

	pos := 0
	tok := tokens[pos]

	if !isTmFuncall(tok) {
		panic("not a `tm_` function")
	}

	pos++
	if tokens[pos].Type != hclsyntax.TokenOParen {
		return nil, 0, errorf("not a funcall")
	}

	matchingParens := 1
	pos++
	for pos < len(tokens) {
		switch tokens[pos].Type {
		case hclsyntax.TokenOParen:
			matchingParens++
		case hclsyntax.TokenCParen:
			matchingParens--
		}

		if matchingParens == 0 {
			break
		}

		pos++
	}

	if matchingParens > 0 || tokens[pos].Type != hclsyntax.TokenCParen {
		return nil, 0, errorf("malformed funcall")
	}

	pos++

	var expr []byte

	for _, et := range tokens[:pos] {
		expr = append(expr, et.Bytes...)
	}

	e, diags := hclsyntax.ParseExpression(expr, "gen.hcl", hcl.Pos{})
	if diags.HasErrors() {
		return nil, 0, errorf("failed to parse expr ('%s'): %v", expr, diags.Error())
	}

	val, err := ctx.Eval(e)
	if err != nil {
		return nil, 0, err
	}

	return hclwrite.TokensForValue(val), pos, nil
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

	if isTmFuncall(tok) {
		return evalTmFuncall(tokens, ctx)
	}

	out := hclwrite.Tokens{tok}

	pos++
	if tokens[pos].Type != hclsyntax.TokenOParen {
		return nil, 0, errorf("not a funcall")
	}

	out = append(out, tokens[pos])

	pos++
	for pos < len(tokens) && tokens[pos].Type != hclsyntax.TokenCParen {
		evaluated, skip, err := evalExpr(false, tokens[pos:], ctx)
		if err != nil {
			return nil, 0, err
		}

		pos += skip
		out = append(out, evaluated...)

		if tokens[pos].Type == hclsyntax.TokenComma {
			out = append(out, tokens[pos])
			pos++
		}
	}

	if pos >= len(tokens) {
		return nil, 0, errorf("malformed funcall")
	}

	if tokens[pos].Type != hclsyntax.TokenCParen {
		panic("bug: funcall not closed")
	}

	out = append(out, tokens[pos])
	pos++

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

func evalInterp(tokens hclwrite.Tokens, ctx *Context) (hclwrite.Tokens, int, error) {
	pos := 0
	tok := tokens[pos]

	if tok.Type != hclsyntax.TokenTemplateInterp {
		panic("unexpected token")
	}

	pos++
	evaluated, skip, err := evalExpr(false, tokens[pos:], ctx)
	if err != nil {
		return nil, 0, err
	}

	pos += skip
	tok = tokens[pos]
	if tok.Type != hclsyntax.TokenTemplateSeqEnd {
		panic("malformed interpolation expression, missing }")
	}

	pos++

	// TODO(i4k):
	//
	// We should emit a `${` and `}` when the expression has non-evaluated parts
	// but there's no easy way of figuring out this without an AST.
	// The naive approach is defined below:
	//   1. check if there's any Operation | Conditional.
	//   2. check if the expression is not fully evaluated.
	//
	// if any of the checks are true, then we need to emit the interp tokens.
	//
	// But there's no way to correctly check 1 without building a AST, as some
	// tokens are used in different grammar constructs (eg.: the ":" is by
	// ConditionalExpr and ForExpr...).
	// So for now we do a lazy (incorrect) check, but this needs to be improved.
	isCombinedExpr := func(tokens hclwrite.Tokens) bool {
		for i := 0; i < len(tokens); i++ {
			switch tokens[i].Type {
			// it's a shame that hclsyntax.TokenType are not integers
			// organized/sorted by kind, so we can check by range...
			case hclsyntax.TokenColon, hclsyntax.TokenQuestion, // conditional
				hclsyntax.TokenAnd, hclsyntax.TokenOr, hclsyntax.TokenBang: // logical
				// TODO(i4k): add the rest.

				return true
			}
		}
		return false
	}

	needsEval := func(tokens hclwrite.Tokens) bool {
		for i := 0; len(tokens) > 2 && i < len(tokens)-2; i++ {
			tok1 := tokens[i]
			tok2 := tokens[i+1]
			tok3 := tokens[i+2]

			if (tok1.Type == hclsyntax.TokenIdent &&
				tok2.Type == hclsyntax.TokenDot &&
				tok3.Type == hclsyntax.TokenIdent) ||
				(tok1.Type == hclsyntax.TokenIdent &&
					tok2.Type == hclsyntax.TokenOParen) {
				return true
			}
		}

		return false
	}

	out := hclwrite.Tokens{}

	shouldEmitInterp := isCombinedExpr(evaluated) || needsEval(evaluated)

	if shouldEmitInterp {
		out = append(out, tokenInterpBegin())
	}

	out = append(out, evaluated...)

	if shouldEmitInterp {
		out = append(out, tokenInterpEnd())
	}

	return out, pos, nil
}

func evalString(tokens hclwrite.Tokens, ctx *Context) (hclwrite.Tokens, int, error) {
	tok := tokens[0]
	if tok.Type != hclsyntax.TokenOQuote {
		return nil, 0, errorf("bug: not a quoted string")
	}

	parts := []hclwrite.Tokens{}

	pos := 1
	for pos < len(tokens) && tokens[pos].Type != hclsyntax.TokenCQuote {
		tok := tokens[pos]
		switch tok.Type {
		case hclsyntax.TokenQuotedLit:
			parts = append(parts, hclwrite.Tokens{tok})
			pos++
		case hclsyntax.TokenTemplateInterp:
			evaluated, skip, err := evalInterp(tokens[pos:], ctx)
			if err != nil {
				return nil, 0, errutil.Chain(ErrInterpolationEval, err)
			}

			parts = append(parts, evaluated)
			pos += skip

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

	pos++

	out := hclwrite.Tokens{
		tokenOQuote(),
	}

	// handles the case of `"${a.b}"` where a.b is not a string.
	if len(parts) == 1 {
		switch parts[0][0].Type {
		case hclsyntax.TokenQuotedLit:
			out = append(out, parts[0]...)
			out = append(out, tokenCQuote())
			return out, pos, nil
		default:
			return parts[0], pos, nil
		}
	}

	var last *hclwrite.Token
	for i := 0; i < len(parts); i++ {
		switch parts[i][0].Type {
		case hclsyntax.TokenOBrace, hclsyntax.TokenOBrack:
			return nil, 0, errutil.Chain(
				ErrInterpolationEval,
				fmt.Errorf("serialization of collection value is not supported"),
			)
		case hclsyntax.TokenQuotedLit:
			if len(parts[i]) > 1 {
				panic("unexpected case")
			}
			if last != nil {
				last.Bytes = append(last.Bytes, parts[i][0].Bytes...)
			} else {
				out = append(out, parts[i]...)
				last = out[len(out)-1]
			}
		case hclsyntax.TokenTemplateInterp:
			out = append(out, parts[i]...)
			last = out[len(out)-1]
		case hclsyntax.TokenNumberLit, hclsyntax.TokenIdent:
			if len(parts[i]) > 1 {
				panic("expects one part")
			}

			if last == nil {
				out = append(out, &hclwrite.Token{
					Type:  hclsyntax.TokenQuotedLit,
					Bytes: parts[i][0].Bytes,
				})
				last = out[len(out)-1]
			} else {
				last.Bytes = append(last.Bytes, parts[i][0].Bytes...)
			}

		case hclsyntax.TokenOQuote:
			if len(parts[i]) != 3 {
				panic(sprintf(
					"unexpected string case: %s (%d)",
					parts[i].Bytes(), len(parts[i])))
			}

			if last == nil {
				out = append(out, &hclwrite.Token{
					Type:  hclsyntax.TokenQuotedLit,
					Bytes: parts[i][1].Bytes,
				})
				last = out[len(out)-1]
			} else {
				last.Bytes = append(last.Bytes, parts[i][1].Bytes...)
			}

		default:
			panic(sprintf("unexpected interpolation type: %s (%s)", parts[i][0].Bytes, parts[i][0].Type))
		}
	}

	out = append(out, tokenCQuote())

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
