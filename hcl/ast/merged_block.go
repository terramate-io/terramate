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

import (
	"sort"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mineiros-io/terramate/errors"
)

// MergedBlock represents a block that spans multiple files.
type MergedBlock struct {
	// Type is the block type (or name).
	Type string

	// Attributes are the block's attributes.
	Attributes Attributes

	// RawOrigins is the list of original blocks that contributed to this block.
	RawOrigins hclsyntax.Blocks

	// Blocks maps block types to merged blocks.
	Blocks map[string]*MergedBlock

	// RawBlocks keeps a map of block type to original blocks.
	RawBlocks map[string]hclsyntax.Blocks
}

// MergedBlocks maps the block name to the MergedBlock.
type MergedBlocks map[string]*MergedBlock

// NewMergedBlock creates a new MergedBlock of type typ.
func NewMergedBlock(typ string) *MergedBlock {
	return &MergedBlock{
		Type:       typ,
		Attributes: make(Attributes),
		Blocks:     make(map[string]*MergedBlock),
		RawBlocks:  make(map[string]hclsyntax.Blocks),
	}
}

// MergeBlock recursively merges the other block into this one.
func (mb *MergedBlock) MergeBlock(fname string, other *hclsyntax.Block) error {
	errs := errors.L()

	// Currently all merged blocks do not support labels.
	// This should not be handled here if changed in the future.
	if len(other.Labels) > 0 {
		errs.Append(errors.E(other.LabelRanges, "block type %q does not support labels"))
	}

	errs.Append(mb.mergeAttrs(fname, other.Body.Attributes))
	errs.Append(mb.mergeBlocks(fname, other.Body.Blocks))
	err := errs.AsError()
	if err == nil {
		mb.RawOrigins = append(mb.RawOrigins, other)
	}
	return err
}

func (mb *MergedBlock) mergeAttrs(origin string, other hclsyntax.Attributes) error {
	errs := errors.L()
	for _, newval := range SortRawAttributes(other) {
		if _, ok := mb.Attributes[newval.Name]; ok {
			errs.Append(errors.E(newval.NameRange,
				"attribute %q redeclared", newval.Name))
			continue
		}
		mb.Attributes[newval.Name] = NewAttribute(origin, newval)
	}
	return errs.AsError()
}

func (mb *MergedBlock) mergeBlocks(origin string, other hclsyntax.Blocks) error {
	errs := errors.L()
	for _, newblock := range other {
		var err error
		if old, ok := mb.Blocks[newblock.Type]; ok {
			err = old.MergeBlock(origin, newblock)
		} else {
			b := NewMergedBlock(newblock.Type)
			err = b.MergeBlock(origin, newblock)
			if err == nil {
				mb.Blocks[newblock.Type] = b
			}
		}

		if err == nil {
			rawBlocks := mb.RawBlocks[newblock.Type]
			mb.RawBlocks[newblock.Type] = append(rawBlocks, newblock)
		}

		errs.Append(err)
	}
	return errs.AsError()
}

func (mb *MergedBlock) ValidateSubBlocks(allowed ...string) error {
	errs := errors.L()

	var blockTypes []string
	for blockType := range mb.RawBlocks {
		blockTypes = append(blockTypes, blockType)
	}

	sort.Strings(blockTypes)

	for _, blockType := range blockTypes {
		rawBlocks := mb.RawBlocks[blockType]

		for _, rawblock := range rawBlocks {
			found := false

			for _, check := range allowed {
				if rawblock.Type == check {
					found = true
				}
			}

			if !found {
				errs.Append(errors.E(rawblock.DefRange(), "unrecognized block %q",
					rawblock.Type))
			}
		}
	}

	return errs.AsError()
}
