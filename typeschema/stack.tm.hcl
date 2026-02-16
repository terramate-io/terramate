// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package typeschema // import \"github.com/terramate-io/terramate/typeschema\""
  description = "package typeschema // import \"github.com/terramate-io/terramate/typeschema\"\n\nPackage typeschema provides type definitions and validation for Terramate\nschemas.\n\nconst KW_ANY = 57351\nconst KW_ANY_OF = 57355\nconst KW_BOOL = 57353\nconst KW_HAS = 57356\nconst KW_LIST = 57346\nconst KW_MAP = 57348\nconst KW_NUMBER = 57354\nconst KW_OBJECT = 57350\nconst KW_SET = 57347\nconst KW_STRING = 57352\nconst KW_TUPLE = 57349\nconst STR = 57357\nfunc IsAnyType(typ Type) bool\nfunc IsCollectionType(typ Type) bool\ntype AnyType struct{}\ntype Lexer struct{ ... }\n    func NewLexer(input string) *Lexer\ntype ListType struct{ ... }\ntype MapType struct{ ... }\ntype MergedObjectType struct{ ... }\ntype NonStrictType struct{ ... }\ntype ObjectType struct{ ... }\ntype ObjectTypeAttribute struct{ ... }\ntype PrimitiveType struct{ ... }\ntype ReferenceType struct{ ... }\ntype Schema struct{ ... }\ntype SchemaNamespaces struct{ ... }\n    func NewSchemaNamespaces() SchemaNamespaces\ntype SetType struct{ ... }\ntype TupleType struct{ ... }\ntype Type interface{ ... }\n    func Parse(typeStr string, inlineAttrs []*ObjectTypeAttribute) (Type, error)\n    func UnwrapValueType(typ Type) Type\ntype VariantType struct{ ... }"
  tags        = ["golang", "typeschema"]
  id          = "ba2878cc-21ca-4262-b4b0-668e7a1cf9df"
}
