// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package ast

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/terramate-io/terramate/hcl/info"
)

// Block is a wrapper to the hclsyntax.Block but with the file origin.
// The hclsyntax.Block.Attributes are converted to hcl.Attributes.
type Block struct {
	Range      info.Range
	Attributes Attributes
	Blocks     []*Block

	*hclsyntax.Block
}

// Blocks is a list of block.
type Blocks []*Block

// NewBlock creates a new block wrapper.
func NewBlock(rootdir string, block *hclsyntax.Block) *Block {
	attrs := make(Attributes)
	for name, val := range block.Body.Attributes {
		attrs[name] = NewAttribute(rootdir, val.AsHCLAttribute())
	}
	var blocks Blocks
	for _, block := range block.Body.Blocks {
		blocks = append(blocks, NewBlock(rootdir, block))
	}
	return &Block{
		Range:      info.NewRange(rootdir, block.Range()),
		Attributes: attrs,
		Blocks:     blocks,
		Block:      block,
	}
}

// NewBlocks creates a Block slice from the raw hclsyntax.Block.
func NewBlocks(rootdir string, rawblocks hclsyntax.Blocks) Blocks {
	var blocks Blocks
	for _, rawblock := range rawblocks {
		blocks = append(blocks, NewBlock(rootdir, rawblock))
	}
	return blocks
}

// LabelRanges computes a range between the first and last label in the case
// the block has labels or a range for the empty space between the block name
// and the open brace.
func (b *Block) LabelRanges() hcl.Range {
	switch n := len(b.Block.LabelRanges); n {
	case 0:
		return hcl.Range{
			Filename: b.Range.HostPath(),

			// returns the range for the caret symbol below:
			// blockname  { ... }
			//          ^ ^
			Start: hcl.Pos{
				Line:   b.Block.TypeRange.Start.Line,
				Column: b.Block.TypeRange.Start.Column + len(b.Type),
				Byte:   b.Block.TypeRange.Start.Byte + len(b.Type),
			},
			End: b.Block.OpenBraceRange.End,
		}
	case 1:
		return b.Block.LabelRanges[0]
	default:
		return hcl.RangeBetween(b.Block.LabelRanges[0], b.Block.LabelRanges[n-1])
	}
}
