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
	"path/filepath"
	"sort"

	"github.com/mineiros-io/terramate/errors"
)

// MergedBlock represents a block that spans multiple files.
type MergedBlock struct {
	// Type is the block type (or name).
	Type string

	// Attributes are the block's attributes.
	Attributes Attributes

	// RawOrigins is the list of original blocks that contributed to this block.
	RawOrigins Blocks

	// Blocks maps block types to merged blocks.
	Blocks map[string]*MergedBlock

	// RawBlocks keeps a map of block type to original blocks.
	RawBlocks map[string]Blocks
}

// MergedBlocks maps the block name to the MergedBlock.
type MergedBlocks map[string]*MergedBlock

// NewMergedBlock creates a new MergedBlock of type typ.
func NewMergedBlock(typ string) *MergedBlock {
	return &MergedBlock{
		Type:       typ,
		Attributes: make(Attributes),
		Blocks:     make(map[string]*MergedBlock),
		RawBlocks:  make(map[string]Blocks),
	}
}

// MergeBlock recursively merges the other block into this one.
func (mb *MergedBlock) MergeBlock(other *Block) error {
	errs := errors.L()

	// Currently all merged blocks do not support labels.
	// This should not be handled here if changed in the future.
	if len(other.Labels) > 0 {
		errs.Append(errors.E(other.LabelRanges, "block type %q does not support labels"))
	}

	errs.Append(mb.mergeAttrs(other.Attributes))
	errs.Append(mb.mergeBlocks(other.Blocks))
	err := errs.AsError()
	if err == nil {
		mb.RawOrigins = append(mb.RawOrigins, other)
	}
	return err
}

func (mb *MergedBlock) mergeAttrs(other Attributes) error {
	errs := errors.L()
	for _, attr := range other.SortedList() {
		if attrVal, ok := mb.Attributes[attr.Name]; ok &&
			sameDir(attrVal.Origin, attr.Origin) {
			errs.Append(errors.E(attr.NameRange,
				"attribute %q redeclared in file %q (first defined in %q)",
				attr.Name,
				attr.Origin, attrVal.Origin))
			continue
		}

		mb.Attributes[attr.Name] = attr
	}
	return errs.AsError()
}

func (mb *MergedBlock) mergeBlocks(other Blocks) error {
	errs := errors.L()
	for _, newblock := range other {
		var err error
		if old, ok := mb.Blocks[newblock.Type]; ok {
			err = old.MergeBlock(newblock)
		} else {
			b := NewMergedBlock(newblock.Type)
			err = b.MergeBlock(newblock)
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

// ValidateSubBlocks checks if the block only has the allowed block types.
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

// AsBlocks returns a Block list from a MergedBlocks.
func (mergedBlocks MergedBlocks) AsBlocks() Blocks {
	var all Blocks
	for _, m := range mergedBlocks {
		all = append(all, m.RawOrigins...)
	}
	return all
}

func sameDir(file1, file2 string) bool {
	return filepath.Dir(file1) == filepath.Dir(file2)
}
