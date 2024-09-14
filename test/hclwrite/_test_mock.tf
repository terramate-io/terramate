// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "hclwrite" {
  content = <<-EOT
package hclwrite // import "github.com/terramate-io/terramate/test/hclwrite"

Package hclwrite aims to provide some facilities making it easier/safer to
generate HCL code for testing purposes. It aims at:

- Close to how HCL is written. - Provide formatted string representation.
- Avoid issues when raw HCL strings are used on tests in general.

It is not a replacement to hclwrite:
https://pkg.go.dev/github.com/hashicorp/hcl/v2/hclwrite It is just easier/nicer
to use on tests + circumvents some limitations like:

-
https://stackoverflow.com/questions/67945463/how-to-use-hcl-write-to-set-expressions-with

We are missing a way to build objects and lists with a similar syntax to blocks.
For now we circumvent this with the AttributeValue function.

func Format(code string) string
type Block struct{ ... }
    func BuildBlock(name string, builders ...BlockBuilder) *Block
    func BuildHCL(builders ...BlockBuilder) *Block
type BlockBuilder interface{ ... }
    func AttributeValue(t *testing.T, name string, expr string) BlockBuilder
    func Boolean(name string, val bool) BlockBuilder
    func Expression(name string, expr string) BlockBuilder
    func Labels(labels ...string) BlockBuilder
    func NumberInt(name string, val int64) BlockBuilder
    func String(name string, val string) BlockBuilder
type BlockBuilderFunc func(*Block)
EOT

  filename = "${path.module}/mock-hclwrite.ignore"
}
