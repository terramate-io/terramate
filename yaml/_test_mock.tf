// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "yaml" {
  content = <<-EOT
package yaml // import "github.com/terramate-io/terramate/yaml"

Package yaml provides YAML encoding and decoding for Terramate data objects.

const ErrSyntax errors.Kind = "syntax error" ...
var NilAttr = Attr[any](nil)
func ConvertFromCty(val cty.Value) (any, error)
func ConvertToHCL(in any, srcRange hcl.Range) (hclsyntax.Expression, error)
func Decode(r io.Reader, dec DocumentDecoder) error
func Encode(e DocumentEncoder, w io.Writer) error
type Attribute[T any] struct{ ... }
    func Attr[T any](v T, args ...any) Attribute[T]
type BundleEnvironment struct{ ... }
type BundleInstance struct{ ... }
type Document struct{ ... }
type DocumentDecoder interface{ ... }
type DocumentEncoder interface{ ... }
type Error struct{ ... }
type Map[T any] []MapItem[T]
type MapItem[T any] struct{ ... }
type Seq[T any] []SeqItem[T]
type SeqItem[T any] struct{ ... }
type StructDecoder interface{ ... }
type StructEncoder interface{ ... }
type TypeInfo interface{ ... }
EOT

  filename = "${path.module}/mock-yaml.ignore"
}
