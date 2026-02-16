// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package yaml // import \"github.com/terramate-io/terramate/yaml\""
  description = "package yaml // import \"github.com/terramate-io/terramate/yaml\"\n\nPackage yaml provides YAML encoding and decoding for Terramate data objects.\n\nconst ErrSyntax errors.Kind = \"syntax error\" ...\nvar NilAttr = Attr[any](nil)\nfunc ConvertFromCty(val cty.Value) (any, error)\nfunc ConvertToHCL(in any, srcRange hcl.Range) (hclsyntax.Expression, error)\nfunc Decode(r io.Reader, dec DocumentDecoder) error\nfunc Encode(e DocumentEncoder, w io.Writer) error\ntype Attribute[T any] struct{ ... }\n    func Attr[T any](v T, args ...any) Attribute[T]\ntype BundleEnvironment struct{ ... }\ntype BundleInstance struct{ ... }\ntype Document struct{ ... }\ntype DocumentDecoder interface{ ... }\ntype DocumentEncoder interface{ ... }\ntype Error struct{ ... }\ntype Map[T any] []MapItem[T]\ntype MapItem[T any] struct{ ... }\ntype Seq[T any] []SeqItem[T]\ntype SeqItem[T any] struct{ ... }\ntype StructDecoder interface{ ... }\ntype StructEncoder interface{ ... }\ntype TypeInfo interface{ ... }"
  tags        = ["golang", "yaml"]
  id          = "504a7ca0-a703-4084-a9d4-f45d9de8882e"
}
