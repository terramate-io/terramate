// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package hcl

import (
	"path/filepath"

	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
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

	mergeHandlers map[string]mergeHandler
}

type mergeHandler func(r *RawConfig, block *ast.Block) error

// NewTopLevelRawConfig returns a new RawConfig object tailored for the
// Terramate top-level attributes and blocks.
func NewTopLevelRawConfig() RawConfig {
	return NewCustomRawConfig(map[string]mergeHandler{
		"terramate":     (*RawConfig).mergeBlock,
		"globals":       (*RawConfig).mergeLabeledBlock,
		"script":        (*RawConfig).addBlock,
		"stack":         (*RawConfig).addBlock,
		"vendor":        (*RawConfig).addBlock,
		"generate_file": (*RawConfig).addBlock,
		"generate_hcl":  (*RawConfig).addBlock,
		"assert":        (*RawConfig).addBlock,
		"import":        func(r *RawConfig, b *ast.Block) error { return nil },
	})
}

// NewCustomRawConfig returns a new customized RawConfig.
func NewCustomRawConfig(handlers map[string]mergeHandler) RawConfig {
	return RawConfig{
		MergedAttributes:  make(ast.Attributes),
		MergedBlocks:      make(ast.MergedBlocks),
		MergedLabelBlocks: make(ast.MergedLabelBlocks),
		mergeHandlers:     handlers,
	}
}

// Copy cfg into a new RawConfig
func (cfg RawConfig) Copy() RawConfig {
	n := NewTopLevelRawConfig()
	_ = n.Merge(cfg)
	return n
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
	handlers := cfg.mergeHandlers

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

		errs.Append(handler(cfg, block))
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
	labelBlock, err := ast.NewLabelBlockType(block.Type, block.Labels)
	if err != nil {
		return errors.E(err, ErrTerramateSchema)
	}
	if other, ok := cfg.MergedLabelBlocks[labelBlock]; ok {
		err := other.MergeBlock(block, true)
		if err != nil {
			return errors.E(ErrTerramateSchema, err)
		}
		return nil
	}

	merged := ast.NewMergedBlock(block.Type, block.Labels)
	err = merged.MergeBlock(block, true)
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
			sameDir(attrVal.Range.HostPath(), attr.Range.HostPath()) {
			errs.Append(errors.E(ErrTerramateSchema,
				attr.NameRange,
				"attribute %q redeclared in file %q (first defined in %q)",
				attr.Name,
				attr.Range.Path(), attrVal.Range.Path()))
			continue
		}

		cfg.MergedAttributes[attr.Name] = attr
	}
	return errs.AsError()
}

func sameDir(file1, file2 string) bool {
	return filepath.Dir(file1) == filepath.Dir(file2)
}
