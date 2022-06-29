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

import (
	"sort"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mineiros-io/terramate/errors"
)

// MergedBlock represents a block that spans multiple files.
type MergedBlock struct {
	Type string

	// Attributes are the block's attributes.
	Attributes Attributes

	// Raw is the list of original blocks that contributed to this block.
	Raw hclsyntax.Blocks

	// SubBlocks are the block's sub blocks.
	SubBlocks map[string]*MergedBlock

	// RawSubBlocks keep the original list of sub blocks.
	RawSubBlocks map[string]hclsyntax.Blocks
}

// MergedBlocks maps the block name to the MergedBlock.
type MergedBlocks map[string]*MergedBlock

// NewMergedBlock creates a new MergedBlock of type typ.
func NewMergedBlock(typ string) *MergedBlock {
	return &MergedBlock{
		Type:         typ,
		Attributes:   make(Attributes),
		SubBlocks:    make(map[string]*MergedBlock),
		RawSubBlocks: make(map[string]hclsyntax.Blocks),
	}
}

// MergeBlock recursively merges the other block into this one.
func (mb *MergedBlock) MergeBlock(fname string, other *hclsyntax.Block) error {
	errs := errors.L()

	// Currently all merged blocks do not support labels.
	// This should not be handled here if changed in the future.
	if len(other.Labels) > 0 {
		errs.Append(errors.E(ErrTerramateSchema, other.LabelRanges,
			"block type %q does not support labels"))
	}

	errs.Append(mb.mergeAttrs(fname, other.Body.Attributes))
	errs.Append(mb.mergeSubBlocks(fname, other.Body.Blocks))
	err := errs.AsError()
	if err == nil {
		mb.Raw = append(mb.Raw, other)
	}
	return err
}

func (mb *MergedBlock) mergeAttrs(origin string, other hclsyntax.Attributes) error {
	for _, newval := range sortAttributes(other) {
		if _, ok := mb.Attributes[newval.Name]; ok {
			return errors.E(ErrHCLSyntax, newval.NameRange,
				"attribute %q redeclared", newval.Name)
		}
		mb.Attributes[newval.Name] = NewAttribute(origin, newval)
	}
	return nil
}

func (mb *MergedBlock) mergeSubBlocks(origin string, other hclsyntax.Blocks) error {
	errs := errors.L()
	for _, newblock := range other {
		var err error
		if old, ok := mb.SubBlocks[newblock.Type]; ok {
			err = old.MergeBlock(origin, newblock)
		} else {
			b := NewMergedBlock(newblock.Type)
			err = b.MergeBlock(origin, newblock)
			if err == nil {
				mb.SubBlocks[newblock.Type] = b
			}
		}

		if err == nil {
			rawBlocks := mb.RawSubBlocks[newblock.Type]
			rawBlocks = append(rawBlocks, newblock)
			mb.RawSubBlocks[newblock.Type] = rawBlocks
		}

		errs.Append(err)
	}
	return errs.AsError()
}

func (mb *MergedBlock) validateSubBlocks(allowed ...string) error {
	errs := errors.L()

	var keys []string
	for key := range mb.RawSubBlocks {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	for _, key := range keys {
		rawBlocks := mb.RawSubBlocks[key]

		for _, rawblock := range rawBlocks {
			found := false

			for _, check := range allowed {
				if rawblock.Type == check {
					found = true
				}
			}

			if !found {
				errs.Append(errors.E(ErrTerramateSchema,
					rawblock.DefRange(), "unrecognized block %q",
					rawblock.Type))
			}
		}
	}

	return errs.AsError()
}
