// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package hclwrite // import \"github.com/terramate-io/terramate/test/hclwrite\""
  description = "package hclwrite // import \"github.com/terramate-io/terramate/test/hclwrite\"\n\nPackage hclwrite aims to provide some facilities making it easier/safer to\ngenerate HCL code for testing purposes. It aims at:\n\n- Close to how HCL is written. - Provide formatted string representation.\n- Avoid issues when raw HCL strings are used on tests in general.\n\nIt is not a replacement to hclwrite:\nhttps://pkg.go.dev/github.com/hashicorp/hcl/v2/hclwrite It is just easier/nicer\nto use on tests + circumvents some limitations like:\n\n-\nhttps://stackoverflow.com/questions/67945463/how-to-use-hcl-write-to-set-expressions-with\n\nWe are missing a way to build objects and lists with a similar syntax to blocks.\nFor now we circumvent this with the AttributeValue function.\n\nfunc Format(code string) string\ntype Block struct{ ... }\n    func BuildBlock(name string, builders ...BlockBuilder) *Block\n    func BuildHCL(builders ...BlockBuilder) *Block\ntype BlockBuilder interface{ ... }\n    func AttributeValue(t *testing.T, name string, expr string) BlockBuilder\n    func Boolean(name string, val bool) BlockBuilder\n    func Expression(name string, expr string) BlockBuilder\n    func Labels(labels ...string) BlockBuilder\n    func NumberInt(name string, val int64) BlockBuilder\n    func String(name string, val string) BlockBuilder\ntype BlockBuilderFunc func(*Block)"
  tags        = ["golang", "hclwrite", "test"]
  id          = "967e7bd6-f743-4a10-bda9-5ca55e8efde5"
}
