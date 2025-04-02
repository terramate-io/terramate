// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package hcl

import (
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
)

// AssertBlockParser is the parser for the "assert" block.
type AssertBlockParser struct {
	asserts *[]AssertConfig
}

// NewAssertBlockParser returns a new parser specification for the "assert" block.
func NewAssertBlockParser(assertsStorage *[]AssertConfig) *AssertBlockParser {
	return &AssertBlockParser{
		asserts: assertsStorage,
	}
}

// Name returns the type of the block.
func (*AssertBlockParser) Name() string {
	return "assert"
}

// Parse parses the "assert" block.
func (a *AssertBlockParser) Parse(p *TerramateParser, block *ast.Block) error {
	if a.asserts == nil {
		a.asserts = &p.ParsedConfig.Asserts
	}
	cfg := AssertConfig{}
	errs := errors.L()

	cfg.Range = block.Range

	errs.Append(checkNoLabels(block))
	errs.Append(checkHasSubBlocks(block))

	for _, attr := range block.Attributes {
		switch attr.Name {
		case "assertion":
			cfg.Assertion = attr.Expr
		case "message":
			cfg.Message = attr.Expr
		case "warning":
			cfg.Warning = attr.Expr
		default:
			errs.Append(errors.E(ErrTerramateSchema, attr.NameRange,
				"unrecognized attribute %s.%s", block.Type, attr.Name,
			))
		}
	}

	if cfg.Assertion == nil {
		errs.Append(errors.E(ErrTerramateSchema, block.Range,
			"assert.assertion is required"))
	}

	if cfg.Message == nil {
		errs.Append(errors.E(ErrTerramateSchema, block.Range,
			"assert.message is required"))
	}

	if err := errs.AsError(); err != nil {
		return err
	}

	*a.asserts = append(*a.asserts, cfg)
	return nil
}
