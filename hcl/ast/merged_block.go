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
	"github.com/mineiros-io/terramate/project"
)

// MergedBlock represents a block that spans multiple files.
type MergedBlock struct {
	// Type is the block type (or name).
	Type BlockType

	// Labels is the comma-separated list of labels
	Labels []string

	// Attributes are the block's attributes.
	Attributes Attributes

	// RawOrigins is the list of original blocks that contributed to this block.
	RawOrigins Blocks

	// Blocks maps block types to merged blocks.
	Blocks map[string]*MergedBlock

	// RawBlocks keeps a map of block type to original blocks.
	RawBlocks map[string]Blocks
}

// BlockType represents a block type.
type BlockType string

// LabelBlockType represents a labelled block type.
type LabelBlockType struct {
	Type      BlockType                       // Type of the block
	Labels    [project.MaxGlobalLabels]string // Labels are the block labels.
	NumLabels int                             // NumLabels is the number of defined labels.
}

// MergedBlocks maps the block name to the MergedBlock.
type MergedBlocks map[string]*MergedBlock

// MergedLabelBlocks maps the block labels/types to the MergedBlock.
type MergedLabelBlocks map[LabelBlockType]*MergedBlock

// NewMergedBlock creates a new MergedBlock of type typ.
func NewMergedBlock(typ string, labels []string) *MergedBlock {
	return &MergedBlock{
		Type:       BlockType(typ),
		Labels:     labels,
		Attributes: make(Attributes),
		Blocks:     make(map[string]*MergedBlock),
		RawBlocks:  make(map[string]Blocks),
	}
}

// NewLabelBlockType returns a new LabelBlockType.
func NewLabelBlockType(typ string, labels []string) (LabelBlockType, error) {
	if len(labels) > project.MaxGlobalLabels {
		return LabelBlockType{}, errors.E(
			"maximum number of global labels is %d but got %d",
			project.MaxGlobalLabels, len(labels),
		)
	}
	return LabelBlockType{
		Type:      BlockType(typ),
		Labels:    newLabels(labels),
		NumLabels: len(labels),
	}, nil
}

// MergeBlock recursively merges the other block into this one.
func (mb *MergedBlock) MergeBlock(other *Block, isLabelled bool) error {
	errs := errors.L()
	if !isLabelled && len(other.Labels) > 0 {
		errs.Append(errors.E(other.LabelRanges, "block type %q does not support labels"))
	} else {
		if !sameLabels(mb.Labels, other.Labels) {
			errs.Append(errors.E(other.LabelRanges,
				"cannot merge blocks of type %q with different set of labels (%s != %s)",
				mb.Labels, other.Labels,
			))
		}
	}

	errs.Append(mb.mergeAttrs(other.Attributes))
	errs.Append(mb.mergeBlocks(other.Blocks, isLabelled))
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

func (mb *MergedBlock) mergeBlocks(other Blocks, isLabelled bool) error {
	errs := errors.L()
	for _, newblock := range other {
		var err error
		if old, ok := mb.Blocks[newblock.Type]; ok {
			err = old.MergeBlock(newblock, isLabelled)
		} else {
			b := NewMergedBlock(newblock.Type, newblock.Labels)
			err = b.MergeBlock(newblock, isLabelled)
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

// AsBlocks returns a Block list from a MergedBlocks.
func (mergedBlocks MergedLabelBlocks) AsBlocks() Blocks {
	var all Blocks
	for _, m := range mergedBlocks {
		all = append(all, m.RawOrigins...)
	}
	return all
}

// AsList returns a list of merged blocks sorted by its label strings.
func (mergedBlocks MergedLabelBlocks) AsList() []*MergedBlock {
	allblocks := []*MergedBlock{}
	for _, mb := range mergedBlocks {
		allblocks = append(allblocks, mb)
	}
	return allblocks
}

func sameDir(file1, file2 string) bool {
	return filepath.Dir(file1) == filepath.Dir(file2)
}

func newLabels(labels []string) [project.MaxGlobalLabels]string {
	var arrlabels [project.MaxGlobalLabels]string
	copy(arrlabels[:], labels)
	return arrlabels
}

func sameLabels(lb1, lb2 []string) bool {
	if len(lb1) != len(lb2) {
		return false
	}
	for i, l := range lb1 {
		if l != lb2[i] {
			return false
		}
	}
	return true
}
