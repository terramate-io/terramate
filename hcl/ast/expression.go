// Copyright 2023 Mineiros GmbH
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

package ast

import (
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/mineiros-io/terramate/errors"
)

// ParseExpression parses the expression str.
func ParseExpression(str string, filename string) (hcl.Expression, error) {
	expr, diags := hclsyntax.ParseExpression([]byte(str), filename, hcl.InitialPos)
	if diags.HasErrors() {
		return nil, errors.E(diags, "parsing expression from bytes")
	}
	return expr, nil
}

// TokensForExpression generates valid tokens for the given expression.
func TokensForExpression(expr hcl.Expression) hclwrite.Tokens {
	tokens := tokensForExpression(expr)
	tokens = append(tokens, eof())
	return tokens
}

func tokensForExpression(expr hcl.Expression) hclwrite.Tokens {
	builder := tokenBuilder{
		tokens: make(hclwrite.Tokens, 0, 32),
	}
	builder.build(expr)
	return builder.tokens
}

type tokenBuilder struct {
	tokens hclwrite.Tokens
}

func (builder *tokenBuilder) add(tokens ...*hclwrite.Token) {
	builder.tokens = append(builder.tokens, tokens...)
}

func (builder *tokenBuilder) build(expr hcl.Expression) {
	switch e := expr.(type) {
	case *hclsyntax.LiteralValueExpr:
		builder.literalTokens(e)
	case *hclsyntax.TemplateExpr:
		builder.templateTokens(e)
	case *hclsyntax.TemplateWrapExpr:
		builder.templateWrapTokens(e)
	case *hclsyntax.BinaryOpExpr:
		builder.binOpTokens(e)
	case *hclsyntax.UnaryOpExpr:
		builder.unaryOpTokens(e)
	case *hclsyntax.TupleConsExpr:
		builder.tupleTokens(e)
	case *hclsyntax.ParenthesesExpr:
		builder.parenExprTokens(e)
	case *hclsyntax.ObjectConsExpr:
		builder.objectTokens(e)
	case *hclsyntax.ObjectConsKeyExpr:
		builder.objectKeyTokens(e)
	case *hclsyntax.ScopeTraversalExpr:
		builder.scopeTraversalTokens(e)
	case *hclsyntax.ConditionalExpr:
		builder.conditionalTokens(e)
	case *hclsyntax.FunctionCallExpr:
		builder.funcallTokens(e)
	case *hclsyntax.IndexExpr:
		builder.indexTokens(e)
	case *hclsyntax.ForExpr:
		builder.forExprTokens(e)
	case *hclsyntax.SplatExpr:
		builder.splatTokens(e)
	case *hclsyntax.AnonSymbolExpr:
		builder.anonSplatTokens(e)
	case *hclsyntax.RelativeTraversalExpr:
		builder.relTraversalTokens(e)
	default:
		panic(fmt.Sprintf("type %T not supported\n", e))
	}
}

func (builder *tokenBuilder) literalTokens(expr *hclsyntax.LiteralValueExpr) {
	builder.add(hclwrite.TokensForValue(expr.Val)...)
}

func (builder *tokenBuilder) templateTokens(tmpl *hclsyntax.TemplateExpr) {
	begin := len(builder.tokens)
	builder.add(oquote())
	var useheredoc bool
	for group, part := range tmpl.Parts {
		tokens := tokensForExpression(part)
		if len(tokens) < 2 || (tokens[0].Type != hclsyntax.TokenOQuote ||
			tokens[len(tokens)-1].Type != hclsyntax.TokenCQuote) {
			builder.add(interpBegin())
			builder.add(tokens...)
			builder.add(interpEnd())
			if group+1 == len(tmpl.Parts) && useheredoc {
				builder.add(nlString())
			}
			continue
		}

		// quoted string
		for _, tok := range tokens[1 : len(tokens)-1] {
			if tok.Type != hclsyntax.TokenQuotedLit {
				builder.add(tok)
				if group+1 == len(tmpl.Parts) && useheredoc {
					builder.add(nlString())
				}
				continue
			}

			// the code below creates multiple TokenStringLit out of a possibly
			// multiline TokenQuotedLit.
			// The implementation must take care of correctly processing only
			// '\', 'n' sequences, ignoring escaped sequences like '\', '\', 'n'.

			var start int
			var end int
			var pos int
			for start < len(tok.Bytes) {
				pos = start
			inner:
				for pos < len(tok.Bytes)-1 {
					if tok.Bytes[pos] != '\\' {
						pos++
						continue
					}

					if tok.Bytes[pos+1] == 'n' {
						break inner
					}
					if tok.Bytes[pos+1] == '\\' {
						pos++
					}
					pos++
				}
				if pos >= len(tok.Bytes)-1 {
					pos = -1
					end = len(tok.Bytes)
				} else {
					useheredoc = true
					end = pos + 2
				}
				strtok := hclwrite.Token{
					Type:  hclsyntax.TokenStringLit,
					Bytes: tok.Bytes[start:end],
				}
				if useheredoc && (pos == -1 && group+1 == len(tmpl.Parts)) {
					strtok.Bytes = append(strtok.Bytes, []byte("\n")...)
				}
				builder.add(&strtok)
				start = end
			}
		}
	}
	if useheredoc {
		builder.tokens[begin] = oheredoc()
		for _, tok := range builder.tokens[begin+1:] {
			if tok.Type == hclsyntax.TokenStringLit {
				tok.Bytes = []byte(renderString(string(tok.Bytes)))
			}
		}
		builder.add(cheredoc())
	} else {
		builder.add(cquote())
	}
}

func (builder *tokenBuilder) templateWrapTokens(tmpl *hclsyntax.TemplateWrapExpr) {
	builder.add(oquote(), interpBegin())
	builder.build(tmpl.Wrapped)
	builder.add(interpEnd(), cquote())
}

func (builder *tokenBuilder) binOpTokens(binop *hclsyntax.BinaryOpExpr) {
	builder.build(binop.LHS)
	var op *hclwrite.Token
	switch binop.Op {
	case hclsyntax.OpAdd:
		op = add()
	case hclsyntax.OpSubtract:
		op = minus()
	case hclsyntax.OpDivide:
		op = slash()
	case hclsyntax.OpMultiply:
		op = star()
	case hclsyntax.OpModulo:
		op = percent()
	case hclsyntax.OpEqual:
		op = equal()
	case hclsyntax.OpNotEqual:
		op = nequal()
	case hclsyntax.OpGreaterThan:
		op = gtr()
	case hclsyntax.OpLessThan:
		op = lss()
	case hclsyntax.OpLessThanOrEqual:
		op = lsseq()
	case hclsyntax.OpGreaterThanOrEqual:
		op = gtreq()
	case hclsyntax.OpLogicalAnd:
		op = and()
	case hclsyntax.OpLogicalOr:
		op = or()
	default:
		panic(fmt.Sprintf("type %T\n", binop.Op))
	}
	op.SpacesBefore = 1
	builder.add(op)
	nexttok := len(builder.tokens)
	builder.build(binop.RHS)
	builder.tokens[nexttok].SpacesBefore = 1
}

func (builder *tokenBuilder) unaryOpTokens(unary *hclsyntax.UnaryOpExpr) {
	switch unary.Op {
	case hclsyntax.OpLogicalNot:
		builder.add(bang())
	case hclsyntax.OpNegate:
		builder.add(minus())
	default:
		panic(fmt.Sprintf("type %T\n", unary.Op))
	}
	builder.build(unary.Val)
}

func (builder *tokenBuilder) parenExprTokens(parenExpr *hclsyntax.ParenthesesExpr) {
	builder.add(oparen())
	builder.build(parenExpr.Expression)
	builder.add(cparen())
}

func (builder *tokenBuilder) tupleTokens(tuple *hclsyntax.TupleConsExpr) {
	builder.add(obrack())
	for i, expr := range tuple.Exprs {
		builder.build(expr)
		if i+1 != len(tuple.Exprs) {
			builder.add(comma())
		}
	}
	builder.add(cbrack())
}

func (builder *tokenBuilder) objectTokens(obj *hclsyntax.ObjectConsExpr) {
	builder.add(obrace())
	if len(obj.Items) > 0 {
		builder.add(nl())
	}
	for _, item := range obj.Items {
		builder.build(item.KeyExpr)
		builder.add(assign(1))
		nexttok := len(builder.tokens)
		builder.build(item.ValueExpr)
		builder.tokens[nexttok].SpacesBefore = 1
		builder.add(nl())
	}
	builder.add(cbrace())
}

func (builder *tokenBuilder) objectKeyTokens(key *hclsyntax.ObjectConsKeyExpr) {
	// TODO(i4k): review the case for key.ForceNonLiteral = true|false
	builder.build(key.Wrapped)
}

func (builder *tokenBuilder) funcallTokens(fn *hclsyntax.FunctionCallExpr) {
	builder.add(ident(fn.Name, 0), oparen())
	for i, expr := range fn.Args {
		builder.build(expr)
		if i+1 != len(fn.Args) {
			builder.add(comma())
		}
	}
	builder.add(cparen())
}

func (builder *tokenBuilder) conditionalTokens(cond *hclsyntax.ConditionalExpr) {
	builder.build(cond.Condition)
	builder.add(question())
	builder.build(cond.TrueResult)
	builder.add(colon())
	builder.build(cond.FalseResult)
}

func (builder *tokenBuilder) forExprTokens(forExpr *hclsyntax.ForExpr) {
	var end *hclwrite.Token
	if forExpr.KeyExpr != nil {
		// it's an object for-expr
		end = cbrace()
		builder.add(obrace(), ident("for", 0))
		if forExpr.KeyVar != "" {
			builder.add(ident(forExpr.KeyVar, 1))
			builder.add(comma())
		}
		builder.add(ident(forExpr.ValVar, 1))
	} else {
		end = cbrack()
		builder.add(obrack(), ident("for", 0))
		if forExpr.KeyVar != "" {
			builder.add(ident(forExpr.KeyVar, 1))
			builder.add(comma())
		}
		builder.add(ident(forExpr.ValVar, 1))
	}
	builder.add(ident("in", 1))
	in := tokensForExpression(forExpr.CollExpr)
	in[0].SpacesBefore = 1
	builder.add(in...)
	builder.add(colon())
	if forExpr.KeyExpr != nil {
		builder.build(forExpr.KeyExpr)
		builder.add(arrow())
		builder.build(forExpr.ValExpr)
	} else {
		builder.build(forExpr.ValExpr)
	}
	if forExpr.CondExpr != nil {
		builder.add(ident("if", 1))
		nexttok := len(builder.tokens)
		builder.build(forExpr.CondExpr)
		builder.tokens[nexttok].SpacesBefore = 1
	}
	builder.add(end)
}

func (builder *tokenBuilder) indexTokens(index *hclsyntax.IndexExpr) {
	builder.build(index.Collection)
	builder.add(obrack())
	builder.build(index.Key)
	builder.add(cbrack())
}

func (builder *tokenBuilder) splatTokens(splat *hclsyntax.SplatExpr) {
	builder.build(splat.Source)
	builder.add(obrack())
	builder.add(star())
	builder.add(cbrack())
	builder.build(splat.Each)
}

func (builder *tokenBuilder) scopeTraversalTokens(scope *hclsyntax.ScopeTraversalExpr) {
	builder.traversalTokens(scope.Traversal)
}

func (builder *tokenBuilder) traversalTokens(traversals hcl.Traversal) {
	for i, traversal := range traversals {
		switch t := traversal.(type) {
		case hcl.TraverseRoot:
			if i > 0 {
				panic("malformed hcl")
			}
			builder.add(ident(t.Name, 0))
		case hcl.TraverseAttr:
			builder.add(dot(), ident(t.Name, 0))
		case hcl.TraverseIndex:
			builder.add(obrack())
			builder.add(hclwrite.TokensForValue(t.Key)...)
			builder.add(cbrack())
		default:
			panic(fmt.Sprintf("type %T\n", t))
		}
	}
}

func (builder *tokenBuilder) relTraversalTokens(traversal *hclsyntax.RelativeTraversalExpr) {
	builder.build(traversal.Source)
	builder.traversalTokens(traversal.Traversal)
}

func (builder *tokenBuilder) anonSplatTokens(anon *hclsyntax.AnonSymbolExpr) {
	// this node is solely used during the splat evaluation.
	// and should generate nothing?
}

func renderString(str string) string {
	type replace struct {
		old string
		new string
	}
	for _, r := range []replace{
		{
			old: "\\t",
			new: "\t",
		},
		{
			old: "\\n",
			new: "\n",
		},
	} {
		str = strings.ReplaceAll(str, r.old, r.new)
	}
	return str
}

func obrace() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenOBrace,
		Bytes: []byte{'{'},
	}
}

func cbrace() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenCBrace,
		Bytes: []byte{'}'},
	}
}

func oparen() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenOParen,
		Bytes: []byte{'('},
	}
}

func cparen() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenCParen,
		Bytes: []byte{')'},
	}
}

func obrack() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenOBrack,
		Bytes: []byte{'['},
	}
}

func cbrack() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenCBrack,
		Bytes: []byte{']'},
	}
}

func star() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenStar,
		Bytes: []byte{'*'},
	}
}

func interpBegin() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenTemplateInterp,
		Bytes: []byte{'$', '{'},
	}
}

func interpEnd() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenTemplateSeqEnd,
		Bytes: []byte{'}'},
	}
}

func percent() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenPercent,
		Bytes: []byte{'%'},
	}
}

func assign(spaces int) *hclwrite.Token {
	return &hclwrite.Token{
		Type:         hclsyntax.TokenEqual,
		Bytes:        []byte{'='},
		SpacesBefore: spaces,
	}
}

func equal() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenEqualOp,
		Bytes: []byte{'=', '='},
	}
}

func nequal() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenNotEqual,
		Bytes: []byte{'!', '='},
	}
}

func gtr() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenGreaterThan,
		Bytes: []byte{'>'},
	}
}

func gtreq() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenGreaterThanEq,
		Bytes: []byte{'>', '='},
	}
}

func arrow() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenFatArrow,
		Bytes: []byte{'=', '>'},
	}
}

func lss() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenLessThan,
		Bytes: []byte{'<'},
	}
}

func lsseq() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenLessThanEq,
		Bytes: []byte{'<', '='},
	}
}

func bang() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenBang,
		Bytes: []byte{'!'},
	}
}

func or() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenOr,
		Bytes: []byte{'|', '|'},
	}
}

func and() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenAnd,
		Bytes: []byte{'&', '&'},
	}
}

func comma() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenComma,
		Bytes: []byte{','},
	}
}

func colon() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenColon,
		Bytes: []byte{':'},
	}
}

func question() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenQuestion,
		Bytes: []byte{'?'},
	}
}

func dot() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenDot,
		Bytes: []byte{'.'},
	}
}

func ident(name string, spaces int) *hclwrite.Token {
	return &hclwrite.Token{
		Type:         hclsyntax.TokenIdent,
		Bytes:        []byte(name),
		SpacesBefore: spaces,
	}
}

func nl() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenNewline,
		Bytes: []byte{'\n'},
	}
}

func nlString() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenQuotedLit,
		Bytes: []byte("\n"),
	}
}

func add() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenPlus,
		Bytes: []byte{'+'},
	}
}

func minus() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenMinus,
		Bytes: []byte{'-'},
	}
}

func slash() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenSlash,
		Bytes: []byte{'/'},
	}
}

func oquote() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenOQuote,
		Bytes: []byte{'"'},
	}
}

func cquote() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenCQuote,
		Bytes: []byte{'"'},
	}
}

func oheredoc() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenOHeredoc,
		Bytes: []byte("<<-EOT\n"),
	}
}

func cheredoc() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenCHeredoc,
		Bytes: []byte("EOT\n"),
	}
}

func eof() *hclwrite.Token {
	return &hclwrite.Token{
		Type: hclsyntax.TokenEOF,
	}
}
