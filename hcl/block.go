package hcl

import "github.com/hashicorp/hcl/v2/hclsyntax"

type Block struct {
	Origin     string
	Attributes Attributes
	Blocks     []*Block

	*hclsyntax.Block
}

type Blocks []*Block

func NewBlock(origin string, block *hclsyntax.Block) *Block {
	attrs := make(Attributes)
	for name, val := range block.Body.Attributes {
		attrs[name] = NewAttribute(origin, val)
	}
	return &Block{
		Origin:     origin,
		Attributes: attrs,
		Block:      block,
	}
}
