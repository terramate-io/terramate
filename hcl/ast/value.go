// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package ast

import (
	"fmt"
	"unicode"
	"unicode/utf8"

	"github.com/terramate-io/hcl/v2/ext/customdecode"
	"github.com/terramate-io/hcl/v2/hclsyntax"
	"github.com/terramate-io/hcl/v2/hclwrite"
	"github.com/terramate-io/terramate/errors"
	"github.com/zclconf/go-cty/cty"
)

// TokensForValue returns the tokens for the provided value.
func TokensForValue(value cty.Value) hclwrite.Tokens {
	builder := tokenBuilder{}
	builder.fromValue(value)
	return builder.tokens
}

func (builder *tokenBuilder) fromValue(val cty.Value) {
	switch typ := val.Type(); {
	case typ == customdecode.ExpressionClosureType:
		closureExpr := val.EncapsulatedValue().(*customdecode.ExpressionClosure)
		builder.fromExpr(closureExpr.Expression)
	case typ == customdecode.ExpressionType:
		builder.fromExpr(customdecode.ExpressionFromVal(val))
	case val.IsNull():
		builder.add(ident("null", 0))
	case !val.IsKnown():
		// Handle unknown values (including dynamic pseudo-type unknowns)
		// These represent values that couldn't be resolved during evaluation
		// Render as null to avoid panics when trying to extract concrete values
		// TODO(snk): Implement tm_slug differently, then we can probably remove this.
		builder.add(ident("null", 0))
	case typ == cty.Bool:
		if val.True() {
			builder.add(ident("true", 0))
		} else {
			builder.add(ident("false", 0))
		}
	case typ == cty.Number:
		builder.add(num(val.AsBigFloat()))
	case typ == cty.String:
		src := renderQuotedString(val.AsString())
		builder.add(oquote())
		if len(src) > 0 {
			builder.add(&hclwrite.Token{
				Type:  hclsyntax.TokenQuotedLit,
				Bytes: src,
			})
		}
		builder.add(cquote())
	case typ.IsMapType() || typ.IsObjectType():
		builder.add(obrace())
		if val.LengthInt() > 0 {
			builder.add(nl())
		}
		i := 0
		for it := val.ElementIterator(); it.Next(); {
			eKey, eVal := it.Element()
			if hclsyntax.ValidIdentifier(eKey.AsString()) {
				builder.add(ident(eKey.AsString(), 0))
			} else {
				builder.fromValue(eKey)
			}
			builder.add(assign(0))
			builder.fromValue(eVal)
			builder.add(nl())
			i++
		}
		builder.add(cbrace())
	case typ.IsListType() || typ.IsSetType() || typ.IsTupleType():
		builder.add(obrack())
		i := 0
		for it := val.ElementIterator(); it.Next(); {
			if i > 0 {
				builder.add(comma())
			}
			_, eVal := it.Element()
			builder.fromValue(eVal)
			i++
		}
		builder.add(cbrack())

	default:
		panic(errors.E(errors.ErrInternal, "formatting for value type %s is not supported", val.Type().FriendlyName()))
	}
}

func renderQuotedString(s string) []byte {
	buf := make([]byte, 0, len(s))
	for i, r := range s {
		switch r {
		case '\n':
			buf = append(buf, '\\', 'n')
		case '\r':
			buf = append(buf, '\\', 'r')
		case '\t':
			buf = append(buf, '\\', 't')
		case '"':
			buf = append(buf, '\\', '"')
		case '\\':
			buf = append(buf, '\\', '\\')
		case '$', '%':
			buf = appendRune(buf, r)
			remain := s[i+1:]
			if len(remain) > 0 && remain[0] == '{' {
				buf = appendRune(buf, r)
			}
		default:
			if !unicode.IsPrint(r) {
				var fmted string
				if r < 65536 {
					fmted = fmt.Sprintf("\\u%04x", r)
				} else {
					fmted = fmt.Sprintf("\\U%08x", r)
				}
				buf = append(buf, fmted...)
			} else {
				buf = appendRune(buf, r)
			}
		}
	}
	return buf
}

func appendRune(b []byte, r rune) []byte {
	l := utf8.RuneLen(r)
	for i := 0; i < l; i++ {
		b = append(b, 0)
	}
	ch := b[len(b)-l:]
	utf8.EncodeRune(ch, r)
	return b
}
