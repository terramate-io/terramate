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
	"github.com/mineiros-io/terramate/errors"
)

// Errors returned when doing partial evaluation.
const (
	ErrPartial             errors.Kind = "partial evaluation failed"
	ErrInterpolation       errors.Kind = "interpolation failed"
	ErrForExprDisallowEval errors.Kind = "`for` expression disallow globals/terramate variables"
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


This implementation is based on HCL v2.11.1 as defined in the "spec" below:
  - https://github.com/hashicorp/hcl/blob/v2.11.1/hclsyntax/spec.md

Motivation:

Terramate supports a HCL code generation feature driven by pure HCL, where
everything with the exception of terramate variables must be copied verbatim
into the generated file. This code is needed because the Hashicorp's hcl library
does not support the partial evaluation of expressions, in other words, given an
expression containing unknown symbols and terramate symbols, the hcl
expression.Value(ctx) fails because all symbols must be populated in the context
and this is obviously not possible for generating arbitrary code.

The solution here involves lexing/scanning the expression bytes into tokens,
then parsing them and evaluating the terramate symbols as they are found,
rewriting the token stream with the primitive values, and keeping everything
else untouched.
*/

type engine struct {
	tokens hclwrite.Tokens
	pos    int
	ctx    *Context

	// evalstack is a stack of evaluated nodes.
	// The engine walks through the token list evaluating them as needed into a
	// separated node struct placed in this stack.
	evalstack nodestack

	multiline int
}

// node represents a grammar node but in terms of its original tokens and
// the rewritten (evaluated) ones.
type node struct {
	original  hclwrite.Tokens
	evaluated hclwrite.Tokens

	hasCond    bool
	hasOp      bool
	hasAcessor bool
}

func newPartialEvalEngine(tokens hclwrite.Tokens, ctx *Context) *engine {
	// we copy the token bytes because the original slice is not safe to
	// be modified.
	newtokens := copytokens(tokens)
	return &engine{
		tokens: newtokens,
		ctx:    ctx,
	}
}

func (e *engine) Eval() (hclwrite.Tokens, error) {
	e.newnode()
	for e.hasTokens() {
		err := e.evalExpr()
		if err != nil {
			return nil, errors.E(ErrPartial, err)
		}
		e.commit()
	}

	if e.evalstack.len() != 1 {
		panic(errorf("eval stack size must always be 1 but got %d", e.evalstack.len()))
	}

	root := e.evalstack.pop()
	return root.evaluated, nil
}

func (e *engine) hasTokens() bool {
	return e.pos < len(e.tokens)
}

func (e *engine) peek() *hclwrite.Token {
	return e.peekn(0)
}

func (e *engine) peekn(n int) *hclwrite.Token {
	return e.tokens[e.pos+n]
}

func (e *engine) assert(want ...hclsyntax.TokenType) {
	tok := e.peek()
	assertToken(tok, want...)
}

func (e *engine) is(want ...hclsyntax.TokenType) bool {
	return isType(e.peek(), want...)
}

func isType(tok *hclwrite.Token, want ...hclsyntax.TokenType) bool {
	found := false

	for _, w := range want {
		if tok.Type == w {
			found = true
			break
		}
	}
	return found
}

func (e *engine) peekAssert(want ...hclsyntax.TokenType) *hclwrite.Token {
	tok := e.peek()
	assertToken(tok, want...)
	return tok
}

func assertToken(got *hclwrite.Token, want ...hclsyntax.TokenType) {
	if !isType(got, want...) {
		panic(errorf("expected token %s but got %s (token bytes: %s)",
			want, got.Type, got.Bytes))
	}
}

// newnode creates and pushes a new node into the evaluation stack.
func (e *engine) newnode() (int, *node) {
	n := &node{}
	e.evalstack.push(n)
	return e.evalstack.len(), n
}

// commit the last node into the previous one.
func (e *engine) commit() {
	if e.evalstack.len() == 1 {
		panic("everything committed")
	}

	tos := e.evalstack.pop()
	merge := e.evalstack.pop()

	merge.pushfrom(tos)
	if tos.hasCond {
		merge.hasCond = true
	}
	if tos.hasOp {
		merge.hasOp = true
	}
	if tos.hasAcessor {
		merge.hasAcessor = true
	}

	e.evalstack.push(merge)
}

func (e *engine) emit() {
	tos := e.evalstack.peek()
	tos.push(e.peek())
	e.pos++
}

func (e *engine) emitn(n int) {
	for i := 0; e.hasTokens() && i < n; i++ {
		e.emit()
	}
}

func (e *engine) emitVariable(v variable) error {
	tos := e.evalstack.peek()
	for i, original := range v.index {
		// Prepare a subengine to evaluate the indexing tokens.
		// The partial eval engine expects the token stream to be EOF terminated.
		index := copytokens(original)
		index = append(index, tokenEOF())
		subengine := newPartialEvalEngine(index, e.ctx)
		subengine.multiline++

		// this will only fail in the case of `<ident>[]` but this would be a
		// syntax error (hopeful) caught by hcl lib.
		tok := subengine.peekn(subengine.skip(0))
		if tok.Type != hclsyntax.TokenStar {
			evaluatedIndex, err := subengine.Eval()
			if err != nil {
				return err
			}

			// remove EOF
			v.index[i] = evaluatedIndex[0 : len(evaluatedIndex)-1]
		}
	}
	tos.pushEvaluated(v.alltokens()...)
	for _, tok := range v.original {
		tos.pushOriginal(tok)
		e.pos++
	}

	return nil
}

func (e *engine) emitTokens(original hclwrite.Tokens, evaluated hclwrite.Tokens) {
	tos := e.evalstack.peek()
	tos.pushEvaluated(evaluated...)
	tos.pushOriginal(original...)
}

func (e *engine) emitnl() {
	for e.hasTokens() && (e.peek().Type == hclsyntax.TokenNewline ||
		e.peek().Type == hclsyntax.TokenComment) {
		e.emit()
	}
}

func (e *engine) emitComments() {
	for e.hasTokens() && e.peek().Type == hclsyntax.TokenComment {
		e.emit()
	}
}

func (e *engine) emitnlparens() {
	if e.multiline > 0 {
		e.emitnl()
	}
}

func (e *engine) skipTokens(from int, tokens ...hclsyntax.TokenType) int {
	i := from
	for i < len(e.tokens) {
		found := false
		for _, t := range tokens {
			if e.peekn(i).Type == t {
				found = true
				break
			}
		}

		if !found {
			break
		}

		i++
	}
	return i
}

func (e *engine) skip(from int) int {
	if e.multiline > 0 {
		return e.multilineSkip(from)
	}
	return e.skipTokens(from, hclsyntax.TokenComment)
}

func (e *engine) multilineSkip(from int) int {
	return e.skipTokens(from, hclsyntax.TokenNewline, hclsyntax.TokenComment)
}

func (e *engine) evalExpr() error {
	_, thisNode := e.newnode()

loop:
	for {
		e.emitnlparens()
		e.emitComments()
		switch t := e.peek().Type; {
		case isUnaryOp(t):
			thisNode.hasOp = true
			e.emit()
		default:
			break loop
		}
	}

	e.emitnlparens()
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
		e.emitnl()

		e.multiline++

		err := e.evalExpr()
		if err != nil {
			return err
		}

		e.commit()
		e.emitnl()

		e.assert(hclsyntax.TokenCParen)

		e.emit()
		e.multiline--
	case hclsyntax.TokenOBrace, hclsyntax.TokenOBrack:
		var err error
		var closeToken hclsyntax.TokenType

		openToken := tok.Type
		if openToken == hclsyntax.TokenOBrace {
			closeToken = hclsyntax.TokenCBrace
		} else {
			closeToken = hclsyntax.TokenCBrack
		}

		pos := e.multilineSkip(1)
		next := e.peekn(pos)
		switch {
		case isForExpr(next):
			err = e.evalForExpr(openToken, closeToken)
		case openToken == hclsyntax.TokenOBrace:
			err = e.evalObject()
		case openToken == hclsyntax.TokenOBrack:
			err = e.evalList()
		default:
			panic(errorf("expected %sFOR or %s but found %s (token bytes: %s)",
				tok.Bytes, tok.Type, next.Type, next.Bytes))
		}

		if err != nil {
			return err
		}

		e.commit()

	case hclsyntax.TokenNumberLit:
		e.emit()
	}

	if e.pos == beginPos {
		panic(errorf("no advance in the position: %s (%s)",
			e.peek().Type, e.tokens[e.pos:].Bytes()))
	}

	e.emitnlparens()
	e.emitComments()

	if !e.hasTokens() {
		return nil
	}

	// exprTerm INDEX,GETATTR,SPLAT (expression acessors)
	tok = e.peek()
	switch tok.Type {
	case hclsyntax.TokenOBrack, hclsyntax.TokenDot:
		err := e.evalAcessors()
		if err != nil {
			return err
		}
		e.commit()
	}

	e.emitnlparens()
	e.emitComments()

	if !e.hasTokens() {
		return nil
	}

	// operation && conditional

	tok = e.peek()
	switch t := tok.Type; {
	case isBinOp(t):
		e.emit()
		e.emitnlparens()
		err := e.evalExpr()
		if err != nil {
			return err
		}
		e.commit()
		thisNode.hasOp = true

	case t == hclsyntax.TokenQuestion:
		e.emit()
		e.emitnlparens()
		err := e.evalExpr()
		if err != nil {
			return err
		}

		e.commit()

		e.assert(hclsyntax.TokenColon)

		e.emit()
		e.emitnlparens()
		err = e.evalExpr()
		if err != nil {
			return err
		}
		e.commit()

		thisNode.hasCond = true
	}

	return nil
}

func (e *engine) evalAcessors() error {
	_, thisNode := e.newnode()

	e.assert(hclsyntax.TokenOBrack, hclsyntax.TokenDot)

	thisNode.hasAcessor = true
	for e.hasTokens() {
		tok := e.peek()
		switch tok.Type {
		default:
			// parsed whole acessor sequence.
			return nil
		case hclsyntax.TokenOBrack:
			err := e.evalIndex()
			if err != nil {
				return err
			}
			e.commit()
		case hclsyntax.TokenDot:
			parsed := false

			pos := e.skip(1)
			next := e.peekn(pos)
			if next.Type == hclsyntax.TokenStar {
				e.emitn(pos + 1)
				parsed = true
			}

			if e.hasTokens() && e.peek().Type == hclsyntax.TokenDot {
				err := e.evalGetAttr()
				if err != nil {
					return err
				}
				e.commit()
				parsed = true
			}

			e.emitnlparens()
			e.emitComments()

			if !parsed {
				panic(errorf("unexpected acessor sequence: %s", e.tokens[e.pos:]))
			}
		}
	}

	return nil
}

func (e *engine) evalIndex() error {
	e.newnode()
	e.multiline++

	defer func() { e.multiline-- }()

	e.assert(hclsyntax.TokenOBrack)
	e.emit()

	e.emitnlparens()
	e.emitComments()
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

	e.emitnlparens()
	e.emitComments()

	e.assert(hclsyntax.TokenCBrack)

	e.emit()
	e.emitnlparens()
	e.emitComments()

	if !e.hasTokens() {
		return nil
	}

	tok := e.peek()
	switch tok.Type {
	case hclsyntax.TokenOBrack, hclsyntax.TokenDot:
		err := e.evalAcessors()
		if err != nil {
			return err
		}
		e.commit()
	}

	return nil
}

func (e *engine) evalGetAttr() error {
	e.newnode()
	e.assert(hclsyntax.TokenDot)

	e.emit()
	e.emitnlparens()
	e.emitComments()

	e.assert(hclsyntax.TokenIdent, hclsyntax.TokenNumberLit)
	e.emit()

	return nil
}

func (e *engine) evalIdent() error {
	e.newnode()
	if !e.canEvaluateIdent() {
		return errorf("malformed code")
	}

	e.assert(hclsyntax.TokenIdent)

	next := e.peekn(e.skip(1))
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

func (e *engine) evalList() error {
	e.newnode()

	e.assert(hclsyntax.TokenOBrack)
	e.emit()

	e.multiline++

	e.emitnlparens()
	for e.hasTokens() && e.peek().Type != hclsyntax.TokenCBrack {
		err := e.evalExpr()
		if err != nil {
			return err
		}

		e.commit()
		tok := e.peek()
		if tok.Type == hclsyntax.TokenComma {
			e.emit()
		}
		e.emitnlparens()
	}

	if !e.hasTokens() {
		panic("malformed list")
	}

	e.assert(hclsyntax.TokenCBrack)
	e.emit()

	e.multiline--
	return nil
}

func (e *engine) evalObject() error {
	e.newnode()

	e.assert(hclsyntax.TokenOBrace)
	e.emit()

	e.emitnl()
	for e.hasTokens() && e.peek().Type != hclsyntax.TokenCBrace {
		err := e.evalExpr()
		if err != nil {
			return err
		}

		e.commit()
		e.emitnl()
		e.assert(hclsyntax.TokenEqual, hclsyntax.TokenColon)
		e.emit()

		err = e.evalExpr()
		if err != nil {
			return err
		}

		e.commit()
		tok := e.peek()
		if tok.Type == hclsyntax.TokenComma {
			e.emit()
		}

		e.emitnl()
	}

	if !e.hasTokens() {
		panic("malformed object")
	}

	e.assert(hclsyntax.TokenCBrace)
	e.emit()
	return nil
}

func (e *engine) evalForExpr(matchOpenType, matchCloseType hclsyntax.TokenType) error {
	_, thisNode := e.newnode()
	// { | [
	e.assert(matchOpenType)
	e.emit()

	e.emitnl()

	// { for
	tok := e.peekAssert(hclsyntax.TokenIdent)
	if string(tok.Bytes) != "for" {
		panic(errorf("expected \"for\" identifier but got %s", tok.Bytes))
	}

	e.emit()
	// { for <ident>,<ident>,...
	for e.hasTokens() && string(e.peek().Bytes) != "in" {
		e.emitnl()
		e.assert(hclsyntax.TokenIdent)

		e.emit()
		e.emitnl()

		tok = e.peek()
		if tok.Type == hclsyntax.TokenComma {
			e.emit()
		}
	}

	e.assert(hclsyntax.TokenIdent)
	e.emit()

	// consume everything and give errors in case of terramate variables being
	// used in the `for`.
	matchingCollectionTokens := 1
	for e.hasTokens() && matchingCollectionTokens > 0 {
		tok = e.peek()
		switch tok.Type {
		case matchOpenType:
			matchingCollectionTokens++
		case matchCloseType:
			matchingCollectionTokens--
		case hclsyntax.TokenQuestion:
			thisNode.hasCond = true
		}
		v, found := e.parseVariable(e.tokens[e.pos:])
		if found {
			if v.isTerramate {
				return errors.E(ErrForExprDisallowEval,
					sprintf("evaluating expression: %s", v.alltokens().Bytes()),
				)
			}

			err := e.emitVariable(v)
			if err != nil {
				return err
			}
		} else {
			e.emit()
		}
	}

	return nil
}

func (e *engine) evalTmFuncall() error {
	e.newnode()
	if len(e.tokens[e.pos:]) < 3 {
		return errorf("not a funcall")
	}

	begin := e.pos
	tok := e.peek()
	if !isTmFuncall(tok) {
		panic(errorf("expected a tm_ funcall but got %s", tok.Bytes))
	}

	pos := e.skip(1)
	assertToken(e.peekn(pos), hclsyntax.TokenOParen)

	matchingParens := 1
	e.pos += pos + 1
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
	for _, part := range e.tokens[begin:e.pos] {
		expr = append(expr, part.Bytes...)
	}

	exprParsed, err := parseExpressionBytes(expr)
	if err != nil {
		return errors.E(err, "evaluating expression: %s", expr)
	}

	val, err := e.ctx.Eval(exprParsed)
	if err != nil {
		return errors.E(err, "evaluating expression: %s", expr)
	}

	evaluated, err := TokensForValue(val)
	if err != nil {
		return err
	}

	e.emitTokens(e.tokens[begin:e.pos], evaluated)
	return nil
}

func (e *engine) evalFuncall() error {
	if len(e.tokens[e.pos:]) < 3 {
		panic(errorf("not a funcall: %s", e.tokens[e.pos:].Bytes()))
	}

	tok := e.peekAssert(hclsyntax.TokenIdent)
	if isTmFuncall(tok) {
		return e.evalTmFuncall()
	}

	e.newnode()
	e.emit()
	e.emitnl()

	e.assert(hclsyntax.TokenOParen)
	e.emit()

	e.emitnl()
	e.multiline++
	for e.hasTokens() && e.peek().Type != hclsyntax.TokenCParen {
		err := e.evalExpr()
		if err != nil {
			return err
		}
		e.commit()
		e.emitnl()

		if e.is(hclsyntax.TokenComma, hclsyntax.TokenEllipsis) {
			e.emit()
		} else {
			e.assert(hclsyntax.TokenCParen)
		}

		e.emitnl()
	}
	e.multiline--

	if !e.hasTokens() {
		panic(errorf("malformed funcall: %s", e.tokens.Bytes()))
	}

	e.assert(hclsyntax.TokenCParen)
	e.emit()
	return nil
}

func (e *engine) evalVar() error {
	e.newnode()
	v, found := e.parseVariable(e.tokens[e.pos:])
	if !found {
		panic(errorf("expect a variable but found %s", e.tokens[e.pos:].Bytes()))
	}

	if !v.isTerramate {
		return e.emitVariable(v)
	}

	var expr []byte
	for _, part := range v.alltokens() {
		expr = append(expr, part.Bytes...)
	}

	data := fmt.Sprintf("%s%s", injectedTokensPrefix, expr)
	exprParsed, diags := hclsyntax.ParseExpression(expr, data, hcl.Pos{
		Line:   1,
		Column: 1,
		Byte:   0,
	})

	if diags.HasErrors() {
		return errorf("failed to parse expr %s: %v", expr, diags.Error())
	}

	val, err := e.ctx.Eval(exprParsed)
	if err != nil {
		return err
	}

	e.emitTokens(e.tokens[e.pos:e.pos+v.size()], hclwrite.TokensForValue(val))
	e.pos += v.size()
	return nil
}

func (e *engine) evalInterp() error {
	e.newnode()

	interpOpen := e.peekAssert(hclsyntax.TokenTemplateInterp)

	e.multiline++
	e.pos++

	err := e.evalExpr()
	if err != nil {
		return err
	}

	e.multiline--

	e.commit()

	interpClose := e.peekAssert(hclsyntax.TokenTemplateSeqEnd)

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
	isCombinedExpr := func(n *node) bool {
		return n.hasCond || n.hasOp || n.hasAcessor
	}

	needsEval := func(n *node) bool {
		if areSameTokens(n.original, n.evaluated) {
			return true
		}

		evaluated := ignorenlc(n.evaluated)

		for i := 0; i < len(evaluated)-2; i++ {
			tok1 := evaluated[i]
			tok2 := evaluated[i+1]
			tok3 := evaluated[i+2]

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

	n := e.evalstack.pop()
	rewritten := &node{}

	shouldEmitInterp := isCombinedExpr(n) || needsEval(n)

	if shouldEmitInterp {
		rewritten.push(interpOpen)
	}

	rewritten.pushfrom(n)

	if shouldEmitInterp {
		rewritten.push(interpClose)
	}

	e.evalstack.push(rewritten)
	return nil
}

func (e *engine) evalString() error {
	stacksize, _ := e.newnode()

	e.assert(hclsyntax.TokenOQuote)

	e.pos++
	for e.hasTokens() && e.peek().Type != hclsyntax.TokenCQuote {
		tok := e.peek()
		switch tok.Type {
		case hclsyntax.TokenQuotedLit:
			e.newnode()
			e.emit()
		case hclsyntax.TokenTemplateInterp:
			err := e.evalInterp()
			if err != nil {
				return errors.E(ErrInterpolation, err)
			}
		default:
			panic(errorf("unexpected token %s (token bytes: %s)", tok.Type, tok.Bytes))
		}
	}

	if !e.hasTokens() {
		panic(errorf("malformed quoted string: %s", e.tokens[e.pos:]))
	}

	e.assert(hclsyntax.TokenCQuote)
	e.pos++

	rewritten := &node{}
	rewritten.push(tokenOQuote())

	// handles the case of a single interpolated object, examples:
	// - "${a.b}"
	// - "${a}"
	// - "${0}"
	// - "${global.something}"
	if e.evalstack.len()-stacksize == 1 {
		e.commit()
		tos := e.evalstack.pop()
		switch tos.evaluated[0].Type {
		case hclsyntax.TokenQuotedLit, hclsyntax.TokenTemplateInterp:
			rewritten.pushfrom(tos)
			rewritten.push(tokenCQuote())
			e.evalstack.push(rewritten)
		default:
			e.evalstack.push(tos)
		}

		return nil
	}

	// handle advanced interpolation cases:
	// - "${0 + 1}" and anything mathing ${<any> <op> <any>}
	// - "${funcall(0)}"
	// - "<string>${<anything>}<string>" and all variants.

	// at this point the stack looks like:
	//
	//                           .
	//                           . (nodePos - 1)
	//                           . scratchPos (this string node)
	//                           . 1st interpolation part
	//                           . 2nd interpolation part
	//                           . nth interpolation part
	//
	// The code below will merge all interpolation parts into this node.

	// we merge subsequent string interpolation into previous (last) TokenQuotedLit.
	var last *hclwrite.Token
	for i := stacksize; i < e.evalstack.len(); i++ {
		elem := e.evalstack.peekn(i)
		elem.evaluated = trimIgnored(elem.evaluated)

		switch elem.evaluated[0].Type {
		case hclsyntax.TokenOBrace, hclsyntax.TokenOBrack:
			return errors.E(
				ErrInterpolation,
				errorf("serialization of collection value is not supported"),
			)
		case hclsyntax.TokenQuotedLit:
			if len(elem.evaluated) > 1 {
				panic(errorf("TokenQuotedLit should be a single token but got %d",
					len(elem.evaluated)))
			}

			rewritten.pushfrom(elem)
			last = rewritten.lastEvaluated()
		case hclsyntax.TokenTemplateInterp:
			rewritten.pushfrom(elem)
		case hclsyntax.TokenNumberLit, hclsyntax.TokenIdent:
			if len(elem.evaluated) > 1 {
				panic(fmt.Errorf("expects one part: %s", elem.evaluated.Bytes()))
			}

			if last == nil {
				rewritten.pushEvaluated(&hclwrite.Token{
					Type:  hclsyntax.TokenQuotedLit,
					Bytes: elem.evaluated[0].Bytes,
				})
				rewritten.pushOriginal(elem.original...)
				last = rewritten.lastEvaluated()
			} else {
				last.Bytes = append(last.Bytes, elem.evaluated[0].Bytes...)
			}

		case hclsyntax.TokenOQuote:
			if len(elem.evaluated) < 2 {
				panic(sprintf(
					"unexpected string case: %s (%d)",
					elem.evaluated.Bytes(),
					len(elem.evaluated)))
			}

			if len(elem.evaluated) == 2 {
				tok := &hclwrite.Token{
					Type: hclsyntax.TokenQuotedLit,
				}
				rewritten.pushEvaluated(tok)
				rewritten.pushOriginal(elem.original...)
				last = rewritten.lastEvaluated()
				continue
			}

			for j := 1; j < len(elem.evaluated)-1; j++ {
				switch elem.evaluated[j].Type {
				case hclsyntax.TokenQuotedLit:
					if last == nil {
						rewritten.pushEvaluated(elem.evaluated[j])
						last = rewritten.lastEvaluated()
					} else {
						last.Bytes = append(last.Bytes, elem.evaluated[j].Bytes...)
					}
				default:
					if last != nil {
						last = nil
					}

					rewritten.pushEvaluated(elem.evaluated[j])
				}
			}

			rewritten.pushOriginal(elem.original...)

		default:
			panic(sprintf("unexpected interpolation type: %s (%s)",
				elem.evaluated.Bytes(), elem.evaluated[0].Type))
		}
	}

	rewritten.push(tokenCQuote())
	e.evalstack.nodes[stacksize-1] = rewritten
	e.evalstack.nodes = e.evalstack.nodes[0:stacksize]

	return nil
}

func (e *engine) parseVariable(tokens hclwrite.Tokens) (v variable, found bool) {
	if len(tokens) < 3 {
		// a variable has at least the format: a.b
		return variable{}, false
	}

	if tokens[0].Type != hclsyntax.TokenIdent {
		return variable{}, false
	}

	pos := e.skip(1)
	wantDot := true
	for pos < len(tokens) {
		pos = e.skip(pos)
		tok := tokens[pos]

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
	v.isTerramate = e.ctx.HasNamespace(nsvar)

	for pos < len(tokens) && tokens[pos].Type == hclsyntax.TokenOBrack {
		index, skip := parseIndexing(tokens[pos:])
		v.index = append(v.index, index)
		pos += skip
	}

	v.original = tokens[0:pos]
	return v, true
}

func parseIndexing(tokens hclwrite.Tokens) (hclwrite.Tokens, int) {
	assertToken(tokens[0], hclsyntax.TokenOBrack)

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
			break
		}

		pos++
	}

	assertToken(tokens[pos], hclsyntax.TokenCBrack)
	return tokens[1:pos], pos + 1
}

func (e *engine) canEvaluateIdent() bool {
	if len(e.tokens[e.pos:]) < 2 {
		return false
	}

	e.assert(hclsyntax.TokenIdent)
	next := e.peekn(e.skip(1))
	return next.Type == hclsyntax.TokenDot || next.Type == hclsyntax.TokenOParen
}

func isCmpOp(t hclsyntax.TokenType) bool {
	switch t {
	case hclsyntax.TokenEqualOp, hclsyntax.TokenNotEqual,
		hclsyntax.TokenLessThan, hclsyntax.TokenLessThanEq,
		hclsyntax.TokenGreaterThan, hclsyntax.TokenGreaterThanEq:
		return true
	}
	return false
}

func isLogicOp(t hclsyntax.TokenType) bool {
	switch t {
	case hclsyntax.TokenOr, hclsyntax.TokenAnd, hclsyntax.TokenBang:
		return true
	}
	return false
}

func isArithOp(t hclsyntax.TokenType) bool {
	switch t {
	case hclsyntax.TokenPlus, hclsyntax.TokenMinus,
		hclsyntax.TokenStar, hclsyntax.TokenSlash, hclsyntax.TokenPercent:
		return true
	}
	return false
}

func isBinOp(t hclsyntax.TokenType) bool {
	return isCmpOp(t) || isArithOp(t) || isLogicOp(t)
}

func isUnaryOp(t hclsyntax.TokenType) bool {
	return t == hclsyntax.TokenBang ||
		t == hclsyntax.TokenMinus ||
		t == hclsyntax.TokenPlus
}

func isForExpr(tok *hclwrite.Token) bool {
	return tok.Type == hclsyntax.TokenIdent && string(tok.Bytes) == "for"
}

func isTmFuncall(tok *hclwrite.Token) bool {
	return tok.Type == hclsyntax.TokenIdent &&
		strings.HasPrefix(string(tok.Bytes), "tm_")
}

func areSameTokens(a, b hclwrite.Tokens) bool {
	if len(a) != len(b) {
		return false
	}

	for i := 0; i < len(a); i++ {
		if b[i].Type != a[i].Type ||
			string(b[i].Bytes) != string(a[i].Bytes) {
			return false
		}
	}
	return true
}

// pushfrom push the original and evaluated tokens from other node into this one.
func (n *node) pushfrom(other *node) {
	n.pushOriginal(other.original...)
	n.pushEvaluated(other.evaluated...)
}

// push the token into both original and evaluated.
func (n *node) push(tok *hclwrite.Token) {
	n.pushOriginal(tok)
	n.pushEvaluated(tok)
}

func (n *node) lastEvaluated() *hclwrite.Token {
	if len(n.evaluated) == 0 {
		panic("bug: no evaluated token")
	}
	return n.evaluated[len(n.evaluated)-1]
}

func (n *node) pushEvaluated(toks ...*hclwrite.Token) {
	n.evaluated = append(n.evaluated, toks...)
}

func (n *node) pushOriginal(toks ...*hclwrite.Token) {
	n.original = append(n.original, toks...)
}

type nodestack struct {
	nodes []*node
}

func (s *nodestack) push(n *node) {
	s.nodes = append(s.nodes, n)
}

func (s *nodestack) pop() *node {
	if len(s.nodes) <= 0 {
		panic("popping on an empty stack")
	}
	top := s.nodes[len(s.nodes)-1]
	s.nodes = s.nodes[:len(s.nodes)-1]
	return top
}

func (s *nodestack) peek() *node {
	return s.peekn(s.len() - 1)
}

func (s *nodestack) peekn(pos int) *node {
	return s.nodes[pos]
}

func (s *nodestack) len() int { return len(s.nodes) }

func trimIgnored(tokens hclwrite.Tokens) hclwrite.Tokens {
	// ignore prefixed and suffixed newlines/comments.

	ignorePos := 0
	for j := 0; j < len(tokens); j++ {
		if tokens[j].Type == hclsyntax.TokenNewline ||
			tokens[j].Type == hclsyntax.TokenComment ||
			tokens[j].Type == hclsyntax.TokenOParen {
			ignorePos = j + 1
		} else {
			break
		}
	}

	tokens = tokens[ignorePos:]
	ignorePos = len(tokens)

	for j := len(tokens) - 1; j >= 0; j-- {
		if tokens[j].Type == hclsyntax.TokenNewline ||
			tokens[j].Type == hclsyntax.TokenComment ||
			tokens[j].Type == hclsyntax.TokenCParen {
			ignorePos = j
		} else {
			break
		}
	}

	return tokens[:ignorePos]
}

func ignorenlc(tokens hclwrite.Tokens) hclwrite.Tokens {
	rest := hclwrite.Tokens{}
	for _, tok := range tokens {
		if tok.Type != hclsyntax.TokenNewline && tok.Type != hclsyntax.TokenComment {
			rest = append(rest, tok)
		}
	}
	return rest
}

func copytokens(tokens hclwrite.Tokens) hclwrite.Tokens {
	var newtokens hclwrite.Tokens
	for _, tok := range tokens {
		newtokens = append(newtokens, copytoken(tok))
	}
	return newtokens
}

func copytoken(tok *hclwrite.Token) *hclwrite.Token {
	newtok := &hclwrite.Token{
		Type:         tok.Type,
		Bytes:        make([]byte, len(tok.Bytes)),
		SpacesBefore: tok.SpacesBefore,
	}
	copy(newtok.Bytes, tok.Bytes)
	return newtok
}

// variable is a low-level representation of a variable in terms of tokens.
type variable struct {
	name hclwrite.Tokens

	// a variable can have nested indexing. eg.: global.a[0][1][global.b][0]
	index []hclwrite.Tokens

	original    hclwrite.Tokens
	isTerramate bool
}

func (v variable) alltokens() hclwrite.Tokens {
	tokens := v.name
	for _, index := range v.index {
		tokens = append(tokens, tokenOBrack())
		tokens = append(tokens, index...)
		tokens = append(tokens, tokenCBrack())
	}
	return tokens
}

func (v variable) size() int {
	sz := len(v.name)
	for _, index := range v.index {
		sz += len(index) + 2 // `[` <tokens> `]`
	}
	return sz
}

var sprintf = fmt.Sprintf
var errorf = fmt.Errorf
