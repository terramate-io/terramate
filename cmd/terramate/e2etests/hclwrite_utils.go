package e2etest

import "github.com/mineiros-io/terramate/test/hclwrite"

var (
	str     = hclwrite.String
	number  = hclwrite.NumberInt
	boolean = hclwrite.Boolean
	labels  = hclwrite.Labels
	expr    = hclwrite.Expression
)

func terramate(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return hclwrite.BuildBlock("terramate", builders...)
}

func config(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return hclwrite.BuildBlock("config", builders...)
}

func generateHCL(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return hclwrite.BuildBlock("generate_hcl", builders...)
}

func content(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return hclwrite.BuildBlock("content", builders...)
}

func globals(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return hclwrite.BuildBlock("globals", builders...)
}
