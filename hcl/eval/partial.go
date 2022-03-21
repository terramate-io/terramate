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

type Engine struct {
	tokens    hclwrite.Tokens
	pos       int
	ctx       *Context
	evaluated []hclwrite.Tokens

	isparen bool
}

func NewEngine(tokens hclwrite.Tokens, ctx *Context) *Engine {
	return &Engine{
		tokens: tokens,
		ctx:    ctx,
	}
}

// PartialEval evaluates only the terramate variable expressions from the list of
// tokens, leaving all the rest as-is. It returns a modified list of tokens with
// no reference to terramate namespaced variables (globals and terramate) and
// functions (tm_ prefixed functions).
func (e *Engine) PartialEval() (hclwrite.Tokens, error) {
	e.scratch()
	for e.hasTokens() {
		err := e.evalExpr()
		if err != nil {
			return nil, errutil.Chain(ErrPartialEval, err)
		}
		e.commit()
	}

	if len(e.evaluated) != 1 {
		panic("invalid number of scratch spaces")
	}

	return e.evaluated[0], nil
}

func (e *Engine) hasTokens() bool {
	return e.pos < len(e.tokens)
}

func (e *Engine) peek() *hclwrite.Token {
	return e.tokens[e.pos]
}

func (e *Engine) peekn(n int) *hclwrite.Token {
	return e.tokens[e.pos+n]
}

func (e *Engine) scratch() int {
	e.evaluated = append(e.evaluated, hclwrite.Tokens{})
	return e.tailpos()
}

func (e *Engine) commit() {
	if e.tailpos() == e.headpos() {
		panic("everything committed")
	}

	mergeat := e.tailpos() - 1
	e.evaluated[mergeat] = append(e.evaluated[mergeat], e.tail()...)
	e.evaluated = e.evaluated[e.headpos() : mergeat+1]
}

func (e *Engine) tail() hclwrite.Tokens { return e.evaluated[e.tailpos()] }

func (e *Engine) emit() {
	e.evaluated[e.tailpos()] = append(e.tail(), e.peek())
	e.pos++
}

func (e *Engine) emitn(n int) {
	for i := 0; e.hasTokens() && i < n; i++ {
		e.emit()
	}
}

func (e *Engine) emitVariable(v variable) {
	e.evaluated[e.tailpos()] = append(e.evaluated[e.tailpos()], v.alltokens()...)
	e.pos += v.size()
}

func (e *Engine) emitTokens(toks ...*hclwrite.Token) {
	e.evaluated[e.tailpos()] = append(e.evaluated[e.tailpos()], toks...)
}

func (e *Engine) emitNewLines() {
	for e.hasTokens() && (e.peek().Type == hclsyntax.TokenNewline ||
		e.peek().Type == hclsyntax.TokenComment) {
		e.emit()
	}
}

func (e *Engine) skipNewLines(from int) int {
	i := from
	for e.hasTokens() && (e.peekn(i).Type == hclsyntax.TokenNewline ||
		e.peekn(i).Type == hclsyntax.TokenComment) {
		i++
	}
	return i
}

func (e *Engine) evalExpr() error {
	//fmt.Printf("parsing: %s %s\n", e.tokens[e.pos:].Bytes(), e.peek().Type)
	e.scratch()

loop:
	for {
		if e.isparen {
			e.emitNewLines()
		}
		switch e.peek().Type {
		case hclsyntax.TokenBang, hclsyntax.TokenMinus, hclsyntax.TokenComment:
			e.emit()
		default:
			break loop
		}
	}

	if e.isparen {
		e.emitNewLines()
	}

	beginPos := e.pos
	tok := e.peek()
	// exprTerm
	switch tok.Type {
	case hclsyntax.TokenEOF:
		e.emit()
	case hclsyntax.TokenOHeredoc:
		e.emit()

		for e.hasTokens() &&
			e.peek().Type != hclsyntax.TokenCHeredoc &&
			e.peek().Type != hclsyntax.TokenEOF { // TODO(i4k): hack to imitate hashicorp lib
			e.emit()
		}
		if !e.hasTokens() {
			panic("expect close heredoc")
		}

		e.emit()
	case hclsyntax.TokenOQuote:
		err := e.evalString()
		if err != nil {
			return err
		}
		e.commit()
	case hclsyntax.TokenIdent:
		switch string(tok.Bytes) {
		case "true", "false", "null":
			e.emit()
		default:
			if e.canEvaluateIdent() {
				err := e.evalIdent()
				if err != nil {
					return err
				}
				e.commit()

			} else {
				e.emit()
			}
		}
	case hclsyntax.TokenOParen:
		e.emit()
		e.emitNewLines()

		e.isparen = true

		err := e.evalExpr()
		if err != nil {
			return err
		}

		e.commit()
		e.emitNewLines()

		if e.peek().Type != hclsyntax.TokenCParen {
			panic(e.peek().Type)
		}

		e.emit()
		e.isparen = false
	case hclsyntax.TokenOBrace, hclsyntax.TokenOBrack:
		var err error

		var closeToken hclsyntax.TokenType

		openToken := tok.Type
		if openToken == hclsyntax.TokenOBrace {
			closeToken = hclsyntax.TokenCBrace
		} else {
			closeToken = hclsyntax.TokenCBrack
		}

		next := e.peekn(1)
		switch {
		case isForExpr(next):
			err = e.evalForExpr(openToken, closeToken)
		case openToken == hclsyntax.TokenOBrace:
			err = e.evalObject()
		case openToken == hclsyntax.TokenOBrack:
			err = e.evalList()
		default:
			panic("unexpected")
		}

		if err != nil {
			return err
		}

		e.commit()

	case hclsyntax.TokenNumberLit:
		e.emit()
	}

	if e.pos == beginPos {
		panic(sprintf("bug: no advance in the position: %s (%s)", e.peek().Type, e.tokens[e.pos:].Bytes()))
	}

	if !e.hasTokens() {
		return nil
	}

	// exprTerm INDEX,GETATTR,SPLAT
	tok = e.peek()
	switch tok.Type {
	case hclsyntax.TokenOBrack:
		err := e.evalIndex()
		if err != nil {
			return err
		}
		e.commit()
	case hclsyntax.TokenDot:
		parsed := false
		for e.peek().Type == hclsyntax.TokenDot {
			next := e.peekn(1)
			if next.Type == hclsyntax.TokenStar {
				e.emitn(2)
				parsed = true
			}

			if e.peek().Type == hclsyntax.TokenDot {
				err := e.evalGetAttr()
				if err != nil {
					return err
				}
				e.commit()
				parsed = true
			}

			if !parsed {
				break
			}
		}
	}

	if e.isparen {
		e.emitNewLines()
	}

	// operation && conditional

	tok = e.peek()
	switch tok.Type {
	case hclsyntax.TokenEqualOp, hclsyntax.TokenNotEqual,
		hclsyntax.TokenLessThan, hclsyntax.TokenLessThanEq,
		hclsyntax.TokenGreaterThan, hclsyntax.TokenGreaterThanEq,
		hclsyntax.TokenOr, hclsyntax.TokenAnd:
		e.emit()
		if e.isparen {
			e.emitNewLines()
		}
		err := e.evalExpr()
		if err != nil {
			return err
		}
		e.commit()

	case hclsyntax.TokenPlus, hclsyntax.TokenMinus,
		hclsyntax.TokenStar, hclsyntax.TokenSlash, hclsyntax.TokenPercent:
		e.emit()
		if e.isparen {
			e.emitNewLines()
		}
		err := e.evalExpr()
		if err != nil {
			return err
		}
		e.commit()
	case hclsyntax.TokenQuestion:
		e.emit()
		if e.isparen {
			e.emitNewLines()
		}
		err := e.evalExpr()
		if err != nil {
			return err
		}

		e.commit()

		if e.peek().Type != hclsyntax.TokenColon {
			panic(errorf(
				"expected `:` but found a %s (%s)",
				e.peek().Bytes, e.peek().Type,
			))
		}

		e.emit()
		if e.isparen {
			e.emitNewLines()
		}
		err = e.evalExpr()
		if err != nil {
			return err
		}
		e.commit()
	}

	return nil
}

func (e *Engine) evalIndex() error {
	e.scratch()
	tok := e.peek()
	if tok.Type != hclsyntax.TokenOBrack {
		panic("expect a '['")
	}

	e.emit()
	if e.peek().Type == hclsyntax.TokenStar {
		// splat: <expr>[*]
		e.emit()
	} else {
		err := e.evalExpr()
		if err != nil {
			return err
		}
		e.commit()
	}

	tok = e.peek()
	if tok.Type != hclsyntax.TokenCBrack {
		panic("expect a ']'")
	}

	e.emit()
	tok = e.peek()
	switch tok.Type {
	case hclsyntax.TokenOBrack:
		err := e.evalIndex()
		if err != nil {
			return err
		}
		e.commit()

	case hclsyntax.TokenDot:
		err := e.evalGetAttr()
		if err != nil {
			return err
		}
		e.commit()
	}

	return nil
}

func (e *Engine) evalGetAttr() error {
	e.scratch()
	if e.peek().Type != hclsyntax.TokenDot {
		panic("expected . IDENT (getAttr)")
	}

	e.emit()
	if e.isparen {
		e.emitNewLines()
	}
	tok := e.peek()
	if tok.Type == hclsyntax.TokenIdent ||
		tok.Type == hclsyntax.TokenNumberLit {
		e.emit()
	} else {
		panic("expect an IDENT or number")
	}

	return nil
}

func (e *Engine) evalIdent() error {
	e.scratch()
	if !e.canEvaluateIdent() {
		return errorf("malformed code")
	}

	tok := e.peek()
	if tok.Type != hclsyntax.TokenIdent {
		panic(errorf("evalIdent: unexpected token '%s' (%s)", tok.Bytes, tok.Type))
	}

	i := e.skipNewLines(1)

	next := e.peekn(i)
	switch next.Type {
	case hclsyntax.TokenDot:
		err := e.evalVar()
		if err != nil {
			return err
		}
		e.commit()
	case hclsyntax.TokenOParen:
		err := e.evalFuncall()
		if err != nil {
			return err
		}
		e.commit()
	default:
		panic("ident cannot be evaluated")
	}

	return nil
}

func (e *Engine) evalList() error {
	e.scratch()
	tok := e.peek()
	if tok.Type != hclsyntax.TokenOBrack {
		panic("bug")
	}

	e.emit()
	for e.hasTokens() && e.peek().Type != hclsyntax.TokenCBrack {
		e.emitNewLines()
		err := e.evalExpr()
		if err != nil {
			return err
		}

		e.commit()
		tok := e.peek()
		if tok.Type == hclsyntax.TokenComma {
			e.emit()
		}
	}

	if !e.hasTokens() {
		panic("malformed list")
	}

	tok = e.peek()
	if tok.Type != hclsyntax.TokenCBrack {
		panic(errorf("malformed list, unexpected %s", tok.Bytes))
	}

	e.emit()
	return nil
}

func (e *Engine) evalObject() error {
	e.scratch()
	tok := e.peek()
	if tok.Type != hclsyntax.TokenOBrace {
		panic("bug")
	}

	e.emit()
	e.emitNewLines()
	for e.hasTokens() && e.peek().Type != hclsyntax.TokenCBrace {
		err := e.evalExpr()
		if err != nil {
			return err
		}

		e.commit()
		e.emitNewLines()
		tok = e.peek()
		if tok.Type != hclsyntax.TokenEqual && tok.Type != hclsyntax.TokenColon {
			panic(errorf("evalObject: unexpected token '%s' (%s)", tok.Bytes, tok.Type))
		}

		e.emit()
		err = e.evalExpr()
		if err != nil {
			return err
		}

		e.commit()
		tok = e.peek()
		if tok.Type == hclsyntax.TokenComma {
			e.emit()
		}

		e.emitNewLines()
	}

	if !e.hasTokens() {
		panic("malformed object")
	}

	tok = e.peek()
	if tok.Type != hclsyntax.TokenCBrace {
		panic(errorf("malformed object, unexpected %s", tok.Bytes))
	}

	e.emit()
	return nil
}

func (e *Engine) evalForExpr(matchOpenType, matchCloseType hclsyntax.TokenType) error {
	e.scratch()
	// { | [
	tok := e.peek()
	if tok.Type != matchOpenType {
		panic(sprintf("evalForExpr: malformed `for` expression: %s", tok.Bytes))
	}

	e.emit()

	// { for
	tok = e.peek()
	if tok.Type != hclsyntax.TokenIdent || string(tok.Bytes) != "for" {
		panic(sprintf("evalForExpr: malformed `for` expression: %s", tok.Bytes))
	}

	e.emit()
	// { for <ident>,<ident>,...
	for e.hasTokens() && string(e.peek().Bytes) != "in" {
		tok = e.peek()
		if tok.Type != hclsyntax.TokenIdent {
			return errorf("invalid `for` expression: found %s", tok.Type)
		}

		e.emit()
		tok = e.peek()
		if tok.Type == hclsyntax.TokenComma {
			e.emit()
		}
	}

	tok = e.peek()
	if tok.Type != hclsyntax.TokenIdent {
		panic(errorf("found the `in` bytes of %s type instead of IDENT", tok.Type))
	}

	e.emit()

	// consume everything and give errors in case of terramate variables being
	// used in the `for`.
	matchingCollectionTokens := 1
	for e.hasTokens() && matchingCollectionTokens > 0 {
		tok = e.peek()
		if tok.Type == matchOpenType {
			matchingCollectionTokens++
		} else if tok.Type == matchCloseType {
			matchingCollectionTokens--
		}
		v, found := e.parseVariable(e.tokens[e.pos:])
		if found {
			if v.isTerramate {
				return errutil.Chain(
					ErrForExprDisallowEval,
					errorf("evaluating expression: %s", v.alltokens().Bytes()),
				)
			}

			e.emitVariable(v)
		} else {
			e.emit()
		}
	}

	return nil
}

func (e *Engine) evalTmFuncall() error {
	e.scratch()
	if len(e.tokens[e.pos:]) < 3 {
		return errorf("not a funcall")
	}

	begin := e.pos
	tok := e.peek()
	if !isTmFuncall(tok) {
		panic("not a `tm_` function")
	}

	if e.peekn(1).Type != hclsyntax.TokenOParen {
		panic(errorf("not a funcall: %s", e.tokens[e.pos:].Bytes()))
	}

	matchingParens := 1
	e.pos += 2
	for e.hasTokens() {
		switch e.peek().Type {
		case hclsyntax.TokenOParen:
			matchingParens++
		case hclsyntax.TokenCParen:
			matchingParens--
		}

		if matchingParens == 0 {
			break
		}

		e.pos++
	}

	if matchingParens > 0 || e.peek().Type != hclsyntax.TokenCParen {
		panic(errorf("malformed funcall: %s", e.tokens.Bytes()))
	}

	e.pos++

	var expr []byte

	for _, et := range e.tokens[begin:e.pos] {
		expr = append(expr, et.Bytes...)
	}

	exprParsed, diags := hclsyntax.ParseExpression(expr, "gen.hcl", hcl.Pos{})
	if diags.HasErrors() {
		return errorf("failed to parse expr ('%s'): %v", expr, diags.Error())
	}

	val, err := e.ctx.Eval(exprParsed)
	if err != nil {
		return err
	}

	e.emitTokens(hclwrite.TokensForValue(val)...)
	return nil
}

func (e *Engine) evalFuncall() error {
	if len(e.tokens[e.pos:]) < 3 {
		return errorf("not a funcall")
	}

	tok := e.peek()
	if tok.Type != hclsyntax.TokenIdent {
		panic(errorf("malformed funcall, not start with IDENT"))
	}

	if isTmFuncall(tok) {
		return e.evalTmFuncall()
	}

	e.scratch()
	e.emit()
	e.emitNewLines()
	if e.peek().Type != hclsyntax.TokenOParen {
		panic(errorf("not a funcall: %s", e.tokens[e.pos:].Bytes()))
	}

	e.emit()
	e.emitNewLines()
	e.isparen = true
	for e.hasTokens() && e.peek().Type != hclsyntax.TokenCParen {
		err := e.evalExpr()
		if err != nil {
			return err
		}
		e.commit()
		e.emitNewLines()

		if e.peek().Type == hclsyntax.TokenComma {
			e.emit()
		} else if e.peek().Type != hclsyntax.TokenCParen {
			panic(errorf("expect a comma or ) but found %s", e.tokens[e.pos:].Bytes()))
		}
		e.emitNewLines()
	}
	e.isparen = false

	if !e.hasTokens() {
		panic(errorf("malformed funcall: %s", e.tokens.Bytes()))
	}

	if e.peek().Type != hclsyntax.TokenCParen {
		panic("bug: funcall not closed")
	}

	e.emit()
	return nil
}

func (e *Engine) evalVar() error {
	e.scratch()
	v, found := e.parseVariable(e.tokens[e.pos:])
	if !found {
		panic("expect a variable")
	}

	if !v.isTerramate {
		e.emitVariable(v)
		return nil
	}

	var expr []byte
	for _, et := range v.alltokens() {
		expr = append(expr, et.Bytes...)
	}

	parsedExpr, diags := hclsyntax.ParseExpression(expr, "gen.hcl", hcl.Pos{})
	if diags.HasErrors() {
		return errorf("failed to parse expr %s: %v", expr, diags.Error())
	}

	val, err := e.ctx.Eval(parsedExpr)
	if err != nil {
		return err
	}

	e.emitTokens(hclwrite.TokensForValue(val)...)
	e.pos += v.size()
	return nil
}

func (e *Engine) evalInterp() error {
	e.scratch()
	tok := e.peek()

	if tok.Type != hclsyntax.TokenTemplateInterp {
		panic("unexpected token")
	}

	e.pos++
	err := e.evalExpr()
	if err != nil {
		return err
	}

	e.commit()

	tok = e.peek()
	if tok.Type != hclsyntax.TokenTemplateSeqEnd {
		panic("malformed interpolation expression, missing }")
	}

	e.pos++

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

	evaluated := e.tail()
	rewritten := hclwrite.Tokens{}

	shouldEmitInterp := isCombinedExpr(evaluated) || needsEval(evaluated)

	if shouldEmitInterp {
		rewritten = append(rewritten, tokenInterpBegin())
	}

	rewritten = append(rewritten, evaluated...)

	if shouldEmitInterp {
		rewritten = append(rewritten, tokenInterpEnd())
	}

	e.evaluated[e.tailpos()] = rewritten
	return nil
}

func (e *Engine) evalString() error {
	scratchPos := e.scratch()
	tok := e.peek()
	if tok.Type != hclsyntax.TokenOQuote {
		return errorf("bug: not a quoted string")
	}

	e.pos++
	for e.hasTokens() && e.peek().Type != hclsyntax.TokenCQuote {
		tok := e.peek()
		switch tok.Type {
		case hclsyntax.TokenQuotedLit:
			e.scratch()
			e.emit()
		case hclsyntax.TokenTemplateInterp:
			err := e.evalInterp()
			if err != nil {
				return errutil.Chain(ErrInterpolationEval, err)
			}
		default:
			panic(errorf("evalString: unexpected token %s (%s)", tok.Bytes, tok.Type))
		}
	}

	if !e.hasTokens() {
		panic(errorf("malformed quoted string %d %d", len(e.tokens[e.pos:]), e.pos))
	}

	tok = e.peek()
	if tok.Type != hclsyntax.TokenCQuote {
		return errorf("malformed quoted string, expected '\"' (close quote)")
	}

	e.pos++
	rewritten := hclwrite.Tokens{
		tokenOQuote(),
	}

	// handles the case of `"${a.b}"` where a.b is not a string.
	if e.tailpos()-scratchPos == 1 {
		e.commit()
		tail := e.tail()
		switch tail[0].Type {
		case hclsyntax.TokenQuotedLit:
			rewritten = append(rewritten, e.tail()...)
			rewritten = append(rewritten, tokenCQuote())
			e.evaluated[e.tailpos()] = rewritten
		}

		return nil
	}

	var last *hclwrite.Token
	for i := scratchPos + 1; i <= e.tailpos(); i++ {
		switch e.evaluated[i][0].Type {
		case hclsyntax.TokenOBrace, hclsyntax.TokenOBrack:
			return errutil.Chain(
				ErrInterpolationEval,
				errorf("serialization of collection value is not supported"),
			)
		case hclsyntax.TokenQuotedLit:
			if len(e.evaluated[i]) > 1 {
				panic("unexpected case")
			}

			rewritten = append(rewritten, e.evaluated[i]...)
			last = rewritten[len(rewritten)-1]
		case hclsyntax.TokenTemplateInterp:
			rewritten = append(rewritten, e.evaluated[i]...)
			last = rewritten[len(rewritten)-1]
		case hclsyntax.TokenNumberLit, hclsyntax.TokenIdent:
			if len(e.evaluated[i]) > 1 {
				panic("expects one part")
			}

			if last == nil {
				rewritten = append(rewritten, &hclwrite.Token{
					Type:  hclsyntax.TokenQuotedLit,
					Bytes: e.evaluated[i][0].Bytes,
				})
				last = rewritten[len(rewritten)-1]
			} else {
				last.Bytes = append(last.Bytes, e.evaluated[i][0].Bytes...)
			}

		case hclsyntax.TokenOQuote:
			if len(e.evaluated[i]) != 3 {
				panic(sprintf(
					"unexpected string case: %s (%d)",
					e.evaluated[i].Bytes(), len(e.evaluated[i])))
			}

			if last == nil {
				rewritten = append(rewritten, &hclwrite.Token{
					Type:  hclsyntax.TokenQuotedLit,
					Bytes: e.evaluated[i][1].Bytes,
				})
				last = rewritten[len(rewritten)-1]
			} else {
				last.Bytes = append(last.Bytes, e.evaluated[i][1].Bytes...)
			}

		default:
			panic(sprintf("unexpected interpolation type: %s (%s)",
				e.evaluated[i][0].Bytes, e.evaluated[i][0].Type))
		}
	}

	rewritten = append(rewritten, tokenCQuote())
	e.evaluated[scratchPos] = rewritten
	e.evaluated = e.evaluated[e.headpos() : scratchPos+1]

	return nil
}

func (e *Engine) parseVariable(tokens hclwrite.Tokens) (v variable, found bool) {
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

		if e.isparen && tok.Type == hclsyntax.TokenNewline {
			pos++
			continue
		}

		if wantDot {
			if tok.Type != hclsyntax.TokenDot {
				break
			}
		} else if tok.Type != hclsyntax.TokenIdent &&
			tok.Type != hclsyntax.TokenNumberLit &&
			tok.Type != hclsyntax.TokenStar {
			break
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
	}

	return v, true
}

func parseIndexing(tokens hclwrite.Tokens) hclwrite.Tokens {
	if tokens[0].Type != hclsyntax.TokenOBrack {
		panic("not an indexing")
	}

	pos := 1
	matchingBracks := 1
	for pos < len(tokens) {
		// here be dragons
		// in other words: we don't validate the index expression, as it's going
		// to be evaluated by hashicorp library anyway (if global/terramate) or
		// ignored otherwise. Let's trust that hcl.Parse() catches all the
		// issues.

		switch tokens[pos].Type {
		case hclsyntax.TokenOBrack:
			matchingBracks++
		case hclsyntax.TokenCBrack:
			matchingBracks--
		}

		if matchingBracks == 0 {
			if tokens[pos+1].Type == hclsyntax.TokenOBrack {
				// beginning of next '[' sequence.
				// this is for the parsing of a.b[<expr][<expr2]...
				matchingBracks++
				pos += 2
				continue
			}

			break
		}

		pos++
	}

	if tokens[pos].Type != hclsyntax.TokenCBrack {
		panic("unexpected")
	}

	return tokens[1:pos]
}

func (e *Engine) canEvaluateIdent() bool {
	if len(e.tokens[e.pos:]) < 2 {
		return false
	}

	if e.peek().Type != hclsyntax.TokenIdent {
		panic("bug: expects an IDENT at pos 0")
	}

	i := e.skipNewLines(1)

	next := e.peekn(i)
	return next.Type == hclsyntax.TokenDot || next.Type == hclsyntax.TokenOParen
}

func (e *Engine) headpos() int {
	if len(e.evaluated) == 0 {
		panic("no evaluated elements")
	}
	return 0
}

func (e *Engine) tailpos() int {
	var pos int
	if len(e.evaluated) > 0 {
		pos = len(e.evaluated) - 1
	} else {
		pos = 0
	}
	return pos
}

func isForExpr(tok *hclwrite.Token) bool {
	return tok.Type == hclsyntax.TokenIdent && string(tok.Bytes) == "for"
}

func isTmFuncall(tok *hclwrite.Token) bool {
	return tok.Type == hclsyntax.TokenIdent &&
		strings.HasPrefix(string(tok.Bytes), "tm_")
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
