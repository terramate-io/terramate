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

package hcl

import "github.com/hashicorp/hcl/v2/hclsyntax"

// Block is a wrapper to the hclsyntax.Block but with the file origin.
// The hclsyntax.Block.Attributes are converted to hcl.Attributes.
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
