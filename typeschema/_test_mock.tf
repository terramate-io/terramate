// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "typeschema" {
  content = <<-EOT
package typeschema // import "github.com/terramate-io/terramate/typeschema"

Package typeschema provides type definitions and validation for Terramate
schemas.

const KW_ANY = 57351
const KW_ANY_OF = 57355
const KW_BOOL = 57353
const KW_HAS = 57356
const KW_LIST = 57346
const KW_MAP = 57348
const KW_NUMBER = 57354
const KW_OBJECT = 57350
const KW_SET = 57347
const KW_STRING = 57352
const KW_TUPLE = 57349
const STR = 57357
func IsAnyType(typ Type) bool
func IsCollectionType(typ Type) bool
type AnyType struct{}
type Lexer struct{ ... }
    func NewLexer(input string) *Lexer
type ListType struct{ ... }
type MapType struct{ ... }
type MergedObjectType struct{ ... }
type NonStrictType struct{ ... }
type ObjectType struct{ ... }
type ObjectTypeAttribute struct{ ... }
type PrimitiveType struct{ ... }
type ReferenceType struct{ ... }
type Schema struct{ ... }
type SchemaNamespaces struct{ ... }
    func NewSchemaNamespaces() SchemaNamespaces
type SetType struct{ ... }
type TupleType struct{ ... }
type Type interface{ ... }
    func Parse(typeStr string, inlineAttrs []*ObjectTypeAttribute) (Type, error)
    func UnwrapValueType(typ Type) Type
type VariantType struct{ ... }
EOT

  filename = "${path.module}/mock-typeschema.ignore"
}
