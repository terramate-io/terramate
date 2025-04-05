// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package hcl

import (
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
)

// InputBlockParser is the parser for the "input" block.
type InputBlockParser struct{}

// NewInputBlockParser returns a new parser specification for the "input" block.
func NewInputBlockParser() *InputBlockParser {
	return &InputBlockParser{}
}

// Name returns the type of the block.
func (i *InputBlockParser) Name() string {
	return "input"
}

// Parse parses the "input" block.
func (i *InputBlockParser) Parse(p *TerramateParser, block *ast.Block) error {
	if !p.hasExperimentalFeature(SharingIsCaringExperimentName) {
		return errors.E(ErrTerramateSchema, block.DefRange(),
			"unrecognized block %q (sharing-is-caring is an experimental feature, it must be enabled before usage with `terramate.config.experiments = [%q]`)", block.Type, SharingIsCaringExperimentName)
	}
	input := Input{
		Range: block.Range,
	}
	errs := errors.L()
	if len(block.Labels) != 1 {
		errs.Append(errors.E(
			ErrTerramateSchema,
			block.Range,
			"expected a single label but %d given",
			len(block.Labels),
		))
	} else {
		input.Name = block.Labels[0]
	}
	for _, attr := range block.Attributes {
		attr := attr
		switch attr.Name {
		case "backend":
			input.Backend = attr.Expr
		case "value":
			input.Value = attr.Expr
		case "from_stack_id":
			input.FromStackID = attr.Expr
		case "sensitive":
			input.Sensitive = attr.Expr
		case "mock":
			input.Mock = attr.Expr
		default:
			errs.Append(errors.E(ErrTerramateSchema, attr.NameRange, "unrecognized attribute %s", attr.Name))
		}
	}
	if input.Backend == nil {
		errs.Append(errors.E(ErrTerramateSchema, block.Range, `attribute "input.backend" is required`))
	}
	if input.FromStackID == nil {
		errs.Append(errors.E(ErrTerramateSchema, block.Range, `attribute "input.from_stack_id" is required`))
	}
	if input.Value == nil {
		errs.Append(errors.E(ErrTerramateSchema, block.Range, `attribute "input.value" is required`))
	}
	if err := errs.AsError(); err != nil {
		return err
	}

	p.ParsedConfig.Inputs = append(p.ParsedConfig.Inputs, input)
	return nil
}
