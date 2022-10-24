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
	"path/filepath"

	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl/ast"
)

// RawConfig is the configuration (attributes and blocks) without schema
// validations.
type RawConfig struct {
	// MergedAttributes are the top-level attributes of all files.
	// This will be available after calling Parse or ParseConfig
	MergedAttributes ast.Attributes

	// MergedBlocks are the merged blocks from all files.
	// This will be available after calling Parse or ParseConfig
	MergedBlocks ast.MergedBlocks

	// MergedLabelBlocks are the labelled merged blocks.
	// This will be available after calling Parse or ParseConfig
	MergedLabelBlocks ast.MergedLabelBlocks

	// UnmergedBlocks are the unmerged blocks from all files.
	// This will be available after calling Parse or ParseConfig
	UnmergedBlocks ast.Blocks
}

// NewRawConfig returns a new RawConfig object.
func NewRawConfig() RawConfig {
	return RawConfig{
		MergedAttributes:  make(ast.Attributes),
		MergedBlocks:      make(ast.MergedBlocks),
		MergedLabelBlocks: make(ast.MergedLabelBlocks),
	}
}

// Copy cfg into a new RawConfig
func (cfg RawConfig) Copy() RawConfig {
	n := NewRawConfig()
	_ = n.Merge(cfg)
	return n
}

func (cfg *RawConfig) mergeHandlers() map[string]mergeHandler {
	return map[string]mergeHandler{
		"terramate":     cfg.mergeBlock,
		"globals":       cfg.mergeLabeledBlock,
		"stack":         cfg.addBlock,
		"vendor":        cfg.addBlock,
		"generate_file": cfg.addBlock,
		"generate_hcl":  cfg.addBlock,
		"assert":        cfg.addBlock,
		"import":        func(b *ast.Block) error { return nil },
	}
}

// Merge the config with the provided other config.
func (cfg *RawConfig) Merge(other RawConfig) error {
	errs := errors.L()
	errs.Append(cfg.mergeAttrs(other.MergedAttributes))
	errs.Append(cfg.mergeBlocks(other.MergedBlocks.AsBlocks()))
	errs.Append(cfg.mergeBlocks(other.MergedLabelBlocks.AsBlocks()))
	errs.Append(cfg.mergeBlocks(other.UnmergedBlocks))
	return errs.AsError()
}

func (cfg *RawConfig) mergeBlocks(blocks ast.Blocks) error {
	handlers := cfg.mergeHandlers()

	errs := errors.L()
	for _, block := range blocks {
		handler, ok := handlers[block.Type]
		if !ok {
			errs.Append(
				errors.E(ErrTerramateSchema, block.DefRange(),
					"unrecognized block %q", block.Type),
			)

			continue
		}

		errs.Append(handler(block))
	}
	return errs.AsError()
}

func (cfg *RawConfig) addBlock(block *ast.Block) error {
	cfg.UnmergedBlocks = append(cfg.UnmergedBlocks, block)
	return nil
}

func (cfg *RawConfig) mergeBlock(block *ast.Block) error {
	if len(block.Labels) > 0 {
		return errors.E("block type %q does not support labels", block.Type)
	}

	if other, ok := cfg.MergedBlocks[block.Type]; ok {
		err := other.MergeBlock(block, false)
		if err != nil {
			return errors.E(ErrTerramateSchema, err)
		}
		return nil
	}

	merged := ast.NewMergedBlock(block.Type, nil)
	err := merged.MergeBlock(block, false)
	if err != nil {
		return errors.E(ErrTerramateSchema, err)
	}
	cfg.MergedBlocks[block.Type] = merged
	return nil
}

func (cfg *RawConfig) mergeLabeledBlock(block *ast.Block) error {
	labelBlock := ast.NewLabelBlockType(block.Type, block.Labels)
	if other, ok := cfg.MergedLabelBlocks[labelBlock]; ok {
		err := other.MergeBlock(block, true)
		if err != nil {
			return errors.E(ErrTerramateSchema, err)
		}
		return nil
	}

	merged := ast.NewMergedBlock(block.Type, block.Labels)
	err := merged.MergeBlock(block, true)
	if err != nil {
		return errors.E(ErrTerramateSchema, err)
	}
	cfg.MergedLabelBlocks[labelBlock] = merged
	return nil
}

func (cfg *RawConfig) mergeAttrs(other ast.Attributes) error {
	errs := errors.L()
	for _, attr := range other.SortedList() {
		if attrVal, ok := cfg.MergedAttributes[attr.Name]; ok &&
			sameDir(attrVal.Origin, attr.Origin) {
			errs.Append(errors.E(ErrTerramateSchema,
				attr.NameRange,
				"attribute %q redeclared in file %q (first defined in %q)",
				attr.Name,
				attr.Origin, attrVal.Origin))
			continue
		}

		cfg.MergedAttributes[attr.Name] = attr
	}
	return errs.AsError()
}

func sameDir(file1, file2 string) bool {
	return filepath.Dir(file1) == filepath.Dir(file2)
}
