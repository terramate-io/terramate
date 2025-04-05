// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package hcl

import (
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
)

// OutputBlockParser is the specification for the output block.
type OutputBlockParser struct{}

// NewOutputBlockParser returns a new parser specification for the "output" block.
func NewOutputBlockParser() *OutputBlockParser {
	return &OutputBlockParser{}
}

// Name returns the type of the block.
func (*OutputBlockParser) Name() string {
	return "output"
}

// Parse parses the "output" block.
func (*OutputBlockParser) Parse(p *TerramateParser, block *ast.Block) error {
	if !p.hasExperimentalFeature(SharingIsCaringExperimentName) {
		return errors.E(ErrTerramateSchema, block.DefRange(),
			"unrecognized block %q (sharing-is-caring is an experimental feature, it must be enabled before usage with `terramate.config.experiments = [%q]`)", block.Type, SharingIsCaringExperimentName)
	}
	output := Output{
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
		output.Name = block.Labels[0]
	}
	for _, attr := range block.Attributes {
		attr := attr
		switch attr.Name {
		case "backend":
			output.Backend = attr.Expr
		case "value":
			output.Value = attr.Expr
		case "description":
			output.Description = attr.Expr
		case "sensitive":
			output.Sensitive = attr.Expr
		default:
			errs.Append(errors.E(ErrTerramateSchema, attr.NameRange, "unrecognized attribute %s", attr.Name))
		}
	}
	if output.Backend == nil {
		errs.Append(errors.E(ErrTerramateSchema, block.Range, `attribute "input.backend" is required`))
	}
	if output.Value == nil {
		errs.Append(errors.E(ErrTerramateSchema, block.Range, `attribute "input.value" is required`))
	}
	if err := errs.AsError(); err != nil {
		return err
	}
	p.ParsedConfig.Outputs = append(p.ParsedConfig.Outputs, output)
	return nil
}
