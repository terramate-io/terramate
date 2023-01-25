package ast

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

func TokensForExpression(expr hclsyntax.Expression) hclwrite.Tokens {
	switch e := expr.(type) {
	case *hclsyntax.LiteralValueExpr:
		return literalTokens(e)
	case *hclsyntax.TemplateExpr:
		tokens := hclwrite.Tokens{}
		for _, part := range e.Parts {
			tokens = append(tokens, TokensForExpression(part)...)
		}
		return tokens
	case *hclsyntax.TupleConsExpr:
		return tupleTokens(e)
	case *hclsyntax.ObjectConsExpr:
		return objectTokens(e)
	case *hclsyntax.ObjectConsKeyExpr:
		return objectKeyTokens(e)
	case *hclsyntax.ScopeTraversalExpr:
		return scopeTravTokens(e)
	case *hclsyntax.FunctionCallExpr:
		return funcallTokens(e)
	default:
		panic(fmt.Sprintf("type %T\n", e))
	}
}

func literalTokens(expr *hclsyntax.LiteralValueExpr) hclwrite.Tokens {
	return hclwrite.TokensForValue(expr.Val)
}

func tupleTokens(tuple *hclsyntax.TupleConsExpr) hclwrite.Tokens {
	tokens := hclwrite.Tokens{
		&hclwrite.Token{
			Type:  hclsyntax.TokenOBrack,
			Bytes: []byte{'['},
		},
	}
	for i, expr := range tuple.Exprs {
		tokens = append(tokens, TokensForExpression(expr)...)
		if i+1 != len(tuple.Exprs) {
			tokens = append(tokens, comma())
		}
	}
	tokens = append(tokens, &hclwrite.Token{
		Type:  hclsyntax.TokenCBrack,
		Bytes: []byte{']'},
	})
	return tokens
}

func objectTokens(obj *hclsyntax.ObjectConsExpr) hclwrite.Tokens {
	tokens := hclwrite.Tokens{
		obrace(),
	}
	if len(obj.Items) > 0 {
		tokens = append(tokens, nl())
	}
	for _, item := range obj.Items {
		tokens = append(tokens, TokensForExpression(item.KeyExpr)...)
		tokens = append(tokens, assign())
		tokens = append(tokens, TokensForExpression(item.ValueExpr)...)
		tokens = append(tokens, nl())
	}
	tokens = append(tokens, cbrace())
	return tokens
}

func objectKeyTokens(key *hclsyntax.ObjectConsKeyExpr) hclwrite.Tokens {
	// TODO(i4k): review the case for key.ForceNonLiteral = true|false
	return TokensForExpression(key.Wrapped)
}

func funcallTokens(fn *hclsyntax.FunctionCallExpr) hclwrite.Tokens {
	tokens := hclwrite.Tokens{
		ident(fn.Name),
		oparen(),
	}

	for i, expr := range fn.Args {
		tokens = append(tokens, TokensForExpression(expr)...)
		if i+1 != len(fn.Args) {
			tokens = append(tokens, comma())
		}
	}

	tokens = append(tokens, cparen())
	return tokens
}

func scopeTravTokens(scope *hclsyntax.ScopeTraversalExpr) hclwrite.Tokens {
	tokens := hclwrite.Tokens{}
	for i, traversal := range scope.Traversal {
		var name string
		switch t := traversal.(type) {
		case hcl.TraverseRoot:
			name = t.Name
		case hcl.TraverseAttr:
			name = t.Name
		default:
			panic(fmt.Sprintf("unsupported traversal: %T", t))
		}
		tokens = append(tokens, ident(name))
		if i+1 != len(scope.Traversal) {
			tokens = append(tokens, dot())
		}
	}

	return tokens
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
		Type:  hclsyntax.TokenOParen,
		Bytes: []byte{')'},
	}
}

func assign() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenEqual,
		Bytes: []byte{'='},
	}
}

func comma() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenComma,
		Bytes: []byte{','},
	}
}

func dot() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenDot,
		Bytes: []byte{'.'},
	}
}

func ident(name string) *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenIdent,
		Bytes: []byte(name),
	}
}

func nl() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenNewline,
		Bytes: []byte{'\n'},
	}
}
