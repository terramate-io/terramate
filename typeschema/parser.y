// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

%{
package typeschema

import (
	"fmt"
	"strings"
	"text/scanner"
)
%}

// Union structure for the parser
%union{
    str   string
    typ   Type
    list  []Type
}

// Keywords
%token KW_LIST
%token KW_SET
%token KW_MAP
%token KW_TUPLE
%token KW_OBJECT
%token KW_ANY
%token KW_STRING
%token KW_BOOL
%token KW_NUMBER
%token KW_ANY_OF
%token KW_HAS
%token KW_BUNDLE

// Terminals
%token <str> STR
%token '(' ')' ',' '.' '+'

// Non-terminals
%type <typ>  type_expr primitive_type list_type set_type map_type tuple_type bundle_type
%type <typ>  any_of_type all_of_type has_type strict_object_ref merge_operand

// Helper lists
%type <str>  qualified_id
%type <list> type_arg_list

%%

top:
    type_expr {
        yylex.(*Lexer).Result = $1
    }

type_expr:
    primitive_type
|   KW_ANY              { $$ = &AnyType{} }
|   KW_OBJECT           { $$ = &ObjectType{} } // 'object' keyword
|   qualified_id        { $$ = &ReferenceType{Name: $1} } // "MyNamespace.Type"
|   list_type
|   set_type
|   map_type
|   tuple_type
|   any_of_type
|   all_of_type
|   has_type
|   bundle_type

any_of_type:
    KW_ANY_OF '(' type_expr ',' type_arg_list ')' {
        $$ = &VariantType{
            Options: append([]Type{$3}, $5...),
        }
    }

all_of_type:
    merge_operand '+' merge_operand {
        $$ = &MergedObjectType{
            Objects: []Type{$1, $3},
        }
    }
|   all_of_type '+' merge_operand {
        merged := $1.(*MergedObjectType)
        $$ = &MergedObjectType{
            Objects: append(merged.Objects, $3),
        }
    }

merge_operand:
    qualified_id { $$ = &ReferenceType{Name: $1} }
|   KW_OBJECT    { $$ = &ObjectType{} }

has_type:
    KW_HAS '(' strict_object_ref ')' {
        $$ = &NonStrictType{Inner: $3}
    }

strict_object_ref:
    qualified_id {
        $$ = &ReferenceType{Name: $1}
    }
|   all_of_type

primitive_type:
    KW_STRING { $$ = &PrimitiveType{Name: "string"} }
|   KW_BOOL   { $$ = &PrimitiveType{Name: "bool"} }
|   KW_NUMBER { $$ = &PrimitiveType{Name: "number"} }

list_type:
    KW_LIST '(' type_expr ')' {
        $$ = &ListType{ValueType: $3}
    }

set_type:
    KW_SET '(' type_expr ')' {
        $$ = &SetType{ValueType: $3}
    }

map_type:
    KW_MAP '(' type_expr ')' {
        $$ = &MapType{ValueType: $3}
    }

tuple_type:
    KW_TUPLE '(' type_arg_list ')' {
        $$ = &TupleType{Elems: $3}
    }

bundle_type:
    KW_BUNDLE '(' STR ')' {
        $$ = &BundleType{ClassID: $3}
    }

// Generic list of types (for anyOf, tuple)
type_arg_list:
    type_arg_list ',' type_expr {
        $$ = append($1, $3)
    }
|   type_expr {
        $$ = []Type{$1}
    }

qualified_id:
    STR
|   STR '.' STR {
        $$ = $1 + "." + $3
    }

%%

type Lexer struct {
	Scanner scanner.Scanner
	Result  Type
	Err     error
}

func NewLexer(input string) *Lexer {
	l := &Lexer{}
	l.Scanner.Init(strings.NewReader(input))
	l.Scanner.Mode = scanner.ScanIdents | scanner.ScanStrings | scanner.ScanInts
	return l
}

func (l *Lexer) Lex(lval *yySymType) int {
	token := l.Scanner.Scan()
	text := l.Scanner.TokenText()

	if l.Err != nil {
		return 0
	}

	switch token {
	case scanner.EOF:
		return 0
	case scanner.Ident:
		switch text {
        // Core Keywords
		case "list":    return KW_LIST
		case "set":     return KW_SET
		case "map":     return KW_MAP
		case "tuple":   return KW_TUPLE
		case "object":  return KW_OBJECT
		case "any":     return KW_ANY
		case "string":  return KW_STRING
		case "bool":    return KW_BOOL
		case "number":  return KW_NUMBER
        case "any_of":  return KW_ANY_OF
        case "has":     return KW_HAS
        case "bundle":  return KW_BUNDLE
        
		default:
			lval.str = text
			return STR
		}
	case scanner.String:
		lval.str = text[1 : len(text)-1]
		return STR
	default:
		return int(text[0])
	}
}

func (l *Lexer) Error(s string) {
	l.Err = fmt.Errorf("%s at position %s", s, l.Scanner.Pos())
}