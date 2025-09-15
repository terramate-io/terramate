// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package hcl

import (
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
)

// GlobalsBlockParser is the parser for the globals block.
type GlobalsBlockParser struct{}

// NewGlobalsBlockParser creates a new globals block parser.
func NewGlobalsBlockParser() *GlobalsBlockParser {
	return &GlobalsBlockParser{}
}

// Name returns the name of the block.
func (g *GlobalsBlockParser) Name() string {
	return "globals"
}

// Parse parses the globals block.
func (g *GlobalsBlockParser) Parse(p *TerramateParser, label ast.LabelBlockType, block *ast.MergedBlock) error {
	if p.ParsedConfig.Globals == nil {
		p.ParsedConfig.Globals = ast.MergedLabelBlocks{}
	}

	p.ParsedConfig.Globals[label] = block
	err := validateGlobals(block)
	if err != nil {
		return errors.E(ErrTerramateSchema, err)
	}
	return nil
}

func validateGlobals(block *ast.MergedBlock) error {
	errs := errors.L()
	if block.Type != "globals" {
		return errors.E(ErrTerramateSchema,
			block.RawOrigins[0].TypeRange, "unexpected block type %q", block.Type)
	}
	errs.Append(block.ValidateSubBlocks("map"))
	for _, raw := range block.RawOrigins {
		for _, subBlock := range raw.Blocks {
			errs.Append(validateMap(subBlock))
		}
	}
	return errs.AsError()
}

// Validate postconditions after parsing.
func (*GlobalsBlockParser) Validate(*TerramateParser) error {
	return nil
}
