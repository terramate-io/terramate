// Copyright 2022 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ast

import "github.com/hashicorp/hcl/v2/hclsyntax"

// Block is a wrapper to the hclsyntax.Block but with the file origin.
// The hclsyntax.Block.Attributes are converted to hcl.Attributes.
type Block struct {
	Range      Range
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
		Range:      NewRange(rootdir, block.Range()),
		Attributes: attrs,
		Blocks:     blocks,
		Block:      block,
	}
}

// NewBlocks creates a Block slice from the raw hclsyntax.Block.
func NewBlocks(origin string, rawblocks hclsyntax.Blocks) Blocks {
	var blocks Blocks
	for _, rawblock := range rawblocks {
		blocks = append(blocks, NewBlock(origin, rawblock))
	}
	return blocks
}
