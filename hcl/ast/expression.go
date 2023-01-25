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
		return scopeTraversalTokens(e)
	case *hclsyntax.FunctionCallExpr:
		return funcallTokens(e)
	case *hclsyntax.IndexExpr:
		return indexTokens(e)
	case *hclsyntax.SplatExpr:
		return splatTokens(e)
	case *hclsyntax.AnonSymbolExpr:
		return anonSplatTokens(e)
	case *hclsyntax.RelativeTraversalExpr:
		return relTraversalTokens(e)
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

func indexTokens(index *hclsyntax.IndexExpr) hclwrite.Tokens {
	tokens := hclwrite.Tokens{}
	tokens = append(tokens, TokensForExpression(index.Collection)...)
	tokens = append(tokens, obrack())
	tokens = append(tokens, TokensForExpression(index.Key)...)
	tokens = append(tokens, cbrack())
	return tokens
}

func splatTokens(splat *hclsyntax.SplatExpr) hclwrite.Tokens {
	tokens := hclwrite.Tokens{}
	tokens = append(tokens, TokensForExpression(splat.Source)...)
	tokens = append(tokens, obrack())
	tokens = append(tokens, star())
	tokens = append(tokens, cbrack())
	tokens = append(tokens, TokensForExpression(splat.Each)...)

	return tokens
}

func anonSplatTokens(anon *hclsyntax.AnonSymbolExpr) hclwrite.Tokens {
	// this node is solely used during the splat evaluation.
	return hclwrite.Tokens{}
}

func scopeTraversalTokens(scope *hclsyntax.ScopeTraversalExpr) hclwrite.Tokens {
	tokens := hclwrite.Tokens{}
	tokens = append(tokens, traversalTokens(scope.Traversal)...)
	return tokens
}

func traversalTokens(traversals hcl.Traversal) hclwrite.Tokens {
	tokens := hclwrite.Tokens{}
	for i, traversal := range traversals {
		switch t := traversal.(type) {
		case hcl.TraverseRoot:
			if i > 0 {
				panic("malformed hcl")
			}
			tokens = append(tokens, ident(t.Name))
		case hcl.TraverseAttr:
			tokens = append(tokens, dot(), ident(t.Name))
		case hcl.TraverseIndex:
			tokens = append(tokens, obrack())
			tokens = append(tokens, hclwrite.TokensForValue(t.Key)...)
			tokens = append(tokens, cbrack())
		default:
			panic(fmt.Sprintf("unsupported traversal: %T", t))
		}
	}
	return tokens
}

func relTraversalTokens(traversal *hclsyntax.RelativeTraversalExpr) hclwrite.Tokens {
	tokens := hclwrite.Tokens{}
	tokens = append(tokens, TokensForExpression(traversal.Source)...)
	tokens = append(tokens, traversalTokens(traversal.Traversal)...)
	//panic(traversal)
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
