// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package ast // import \"github.com/terramate-io/terramate/hcl/ast\""
  description = "package ast // import \"github.com/terramate-io/terramate/hcl/ast\"\n\nPackage ast provides low level parsing facilities for HCL configuration.\nIt includes extra features like block merging, specific to Terramate needs.\n\nfunc AsHCLAttributes(syntaxAttrs hclsyntax.Attributes) hhcl.Attributes\nfunc CloneExpr(expr hclsyntax.Expression) hclsyntax.Expression\nfunc ParseExpression(str string, filename string) (hcl.Expression, error)\nfunc SortRawAttributes(attrs hhcl.Attributes) []*hhcl.Attribute\nfunc TokensForExpression(expr hcl.Expression) hclwrite.Tokens\nfunc TokensForValue(value cty.Value) hclwrite.Tokens\ntype Attribute struct{ ... }\n    func NewAttribute(rootdir string, val *hhcl.Attribute) Attribute\ntype AttributeSlice []Attribute\ntype Attributes map[string]Attribute\n    func NewAttributes(rootdir string, rawAttrs hhcl.Attributes) Attributes\ntype Block struct{ ... }\n    func NewBlock(rootdir string, block *hclsyntax.Block) *Block\ntype BlockType string\ntype Blocks []*Block\n    func NewBlocks(rootdir string, rawblocks hclsyntax.Blocks) Blocks\ntype CloneExpression struct{ ... }\ntype LabelBlockType struct{ ... }\n    func NewEmptyLabelBlockType(typ string) LabelBlockType\n    func NewLabelBlockType(typ string, labels []string) (LabelBlockType, error)\ntype MergedBlock struct{ ... }\n    func NewMergedBlock(typ string, labels []string) *MergedBlock\ntype MergedBlocks map[string]*MergedBlock\ntype MergedLabelBlocks map[LabelBlockType]*MergedBlock"
  tags        = ["ast", "golang", "hcl"]
  id          = "d4394e8f-4dfc-4d46-b5a5-a1edea55fab4"
}
