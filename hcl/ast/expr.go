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
	"github.com/hashicorp/hcl/v2/ext/customdecode"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/mineiros-io/terramate/errors"
	"github.com/zclconf/go-cty/cty"
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
	tokens[0].SpacesBefore = 0
	tokens = append(tokens, eof())
	return tokens
}

// TokensForValue returns the tokens for the provided value.
func TokensForValue(value cty.Value) hclwrite.Tokens {
	if value.Type() == customdecode.ExpressionClosureType {
		closureExpr := value.EncapsulatedValue().(*customdecode.ExpressionClosure)
		return TokensForExpression(closureExpr.Expression)
	} else if value.Type() == customdecode.ExpressionType {
		return TokensForExpression(customdecode.ExpressionFromVal(value))
	}
	return hclwrite.TokensForValue(value)
}

func tokensForExpression(expr hcl.Expression) hclwrite.Tokens {
	builder := tokenBuilder{}
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
	builder.add(TokensForValue(expr.Val)...)
}

func (builder *tokenBuilder) templateTokens(tmpl *hclsyntax.TemplateExpr) {
	canMergeTemplate := func(tokens hclwrite.Tokens) bool {
		if len(tokens) > 1 {
			return false
		}
		toktype := tokens[0].Type
		tokstr := string(tokens[0].Bytes)
		switch toktype {
		case hclsyntax.TokenNumberLit, hclsyntax.TokenStringLit:
			return true
		case hclsyntax.TokenIdent:
			if tokstr == "true" || tokstr == "false" {
				return true
			}
		}
		return false
	}
	begin := len(builder.tokens)
	builder.add(oquote())
	for _, part := range tmpl.Parts {
		tokens := tokensForExpression(part)
		if tokens[0].Type != hclsyntax.TokenOQuote {
			addInterp := !canMergeTemplate(tokens)
			if addInterp {
				builder.add(interpBegin())
			}
			builder.add(tokens...)
			if addInterp {
				builder.add(interpEnd())
			}
			continue
		}

		// quoted string
		for _, tok := range tokens[1 : len(tokens)-1] {
			if tok.Type != hclsyntax.TokenQuotedLit {
				builder.add(tok)
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
				for pos < len(tok.Bytes)-1 {
					if tok.Bytes[pos] != '\\' {
						pos++
						continue
					}

					if tok.Bytes[pos+1] == 'n' {
						break
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
					end = pos + 2
				}
				strtok := hclwrite.Token{
					Type:  hclsyntax.TokenStringLit,
					Bytes: tok.Bytes[start:end],
				}
				builder.add(&strtok)
				start = end
			}
		}
	}
	last := builder.tokens[len(builder.tokens)-1]
	if last.Type == hclsyntax.TokenStringLit &&
		isHeredoc(last.Bytes) {
		builder.tokens[begin] = oheredoc()
		for _, tok := range builder.tokens[begin+1:] {
			if tok.Type == hclsyntax.TokenStringLit {
				tok.Bytes = renderString(tok.Bytes)
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
	case hclsyntax.OpDivide:
		op = slash()
	case hclsyntax.OpEqual:
		op = equal()
	case hclsyntax.OpGreaterThan:
		op = gtr()
	case hclsyntax.OpGreaterThanOrEqual:
		op = gtreq()
	case hclsyntax.OpLessThan:
		op = lss()
	case hclsyntax.OpLessThanOrEqual:
		op = lsseq()
	case hclsyntax.OpLogicalAnd:
		op = and()
	case hclsyntax.OpLogicalOr:
		op = or()
	case hclsyntax.OpModulo:
		op = percent()
	case hclsyntax.OpMultiply:
		op = star()
	case hclsyntax.OpNotEqual:
		op = nequal()
	case hclsyntax.OpSubtract:
		op = minus()
	default:
		panic(fmt.Sprintf("unexpected binary operation %+v\n", binop.Op))
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
		panic(fmt.Sprintf("value %+v is unexpected\n", unary.Op))
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
	if fn.ExpandFinal {
		builder.add(ellipsis())
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
	if forExpr.Group {
		builder.add(ellipsis())
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

func (builder *tokenBuilder) anonSplatTokens(_ *hclsyntax.AnonSymbolExpr) {
	// this node is solely used during the splat evaluation.
	// and should generate nothing?
}

// isHeredoc checks if the bytes can be represented as a heredoc string.
// A valid heredoc must end with a newline and should only have printable
// characters. (\r and \u sequences from the non-printable range).
// Note: All \uXXXX and \uXXXXXXXX sequences with printable characters should
// have been rendered already by the HCL library, so at this point we only need
// to check for `\r` and `\u`.
func isHeredoc(bytes []byte) bool {
	last := len(bytes) - 1
	var heredoc bool
	if len(bytes) > 1 {
		heredoc = bytes[last] == 'n' && bytes[last-1] == '\\'
	}

	if heredoc {
		// `if`s below disambiguate the sequences:
		//   - \\\n    (must return a heredoc)
		//   - \\n     (must *NOT* return a heredoc)
		if len(bytes) > 3 {
			heredoc = bytes[last-2] != '\\' || bytes[last-3] == '\\'
		} else if len(bytes) > 2 {
			heredoc = bytes[last-2] != '\\'
		}
	}

	if !heredoc {
		return false
	}

	// checks for non-printable escape sequences
	for i, b := range bytes {
		if i == 0 || bytes[i-1] != '\\' {
			continue
		}
		switch b {
		case 'u', 'r':
			return false
		}
	}
	return true
}

func renderString(bytes []byte) []byte {
	type replace struct {
		old string
		new string
	}
	str := string(bytes)
	for _, r := range []replace{
		{
			old: "\\\\",
			new: "\\",
		},
		{
			old: "\\t",
			new: "\t",
		},
		{
			old: "\\n",
			new: "\n",
		},
		{
			old: `\"`,
			new: `"`,
		},
	} {
		str = strings.ReplaceAll(str, r.old, r.new)
	}
	return []byte(str)
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

func ellipsis() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenEllipsis,
		Bytes: []byte("..."),
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
