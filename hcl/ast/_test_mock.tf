// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "ast" {
  content = <<-EOT
package ast // import "github.com/terramate-io/terramate/hcl/ast"

Package ast provides low level parsing facilities for HCL configuration.
It includes extra features like block merging, specific to Terramate needs.

func AsHCLAttributes(syntaxAttrs hclsyntax.Attributes) hhcl.Attributes
func CloneExpr(expr hclsyntax.Expression) hclsyntax.Expression
func ParseExpression(str string, filename string) (hcl.Expression, error)
func SortRawAttributes(attrs hhcl.Attributes) []*hhcl.Attribute
func TokensForExpression(expr hcl.Expression) hclwrite.Tokens
func TokensForValue(value cty.Value) hclwrite.Tokens
type Attribute struct{ ... }
    func NewAttribute(rootdir string, val *hhcl.Attribute) Attribute
type AttributeSlice []Attribute
type Attributes map[string]Attribute
    func NewAttributes(rootdir string, rawAttrs hhcl.Attributes) Attributes
type Block struct{ ... }
    func NewBlock(rootdir string, block *hclsyntax.Block) *Block
type BlockType string
type Blocks []*Block
    func NewBlocks(rootdir string, rawblocks hclsyntax.Blocks) Blocks
type CloneExpression struct{ ... }
type LabelBlockType struct{ ... }
    func NewEmptyLabelBlockType(typ string) LabelBlockType
    func NewLabelBlockType(typ string, labels []string) (LabelBlockType, error)
type MergedBlock struct{ ... }
    func NewMergedBlock(typ string, labels []string) *MergedBlock
type MergedBlocks map[string]*MergedBlock
type MergedLabelBlocks map[LabelBlockType]*MergedBlock
EOT

  filename = "${path.module}/mock-ast.ignore"
}
