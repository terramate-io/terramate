// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package hcl

import (
	"strings"

	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
)

// EnvironmentBlockParser is a parser for the top-level `environment` block.
type EnvironmentBlockParser struct{}

// Name returns the name of the block.
func (p *EnvironmentBlockParser) Name() string {
	return "environment"
}

// NewEnvironmentBlockParser returns a new EnvironmentBlockParser.
func NewEnvironmentBlockParser() *EnvironmentBlockParser {
	return &EnvironmentBlockParser{}
}

// Parse parses the `environment` block.
func (*EnvironmentBlockParser) Parse(p *TerramateParser, block *ast.Block) error {
	if len(block.Labels) != 0 {
		return errors.E(
			ErrTerramateSchema,
			block.LabelRanges(),
			"environment block must have no labels but got %d",
			len(block.Labels),
		)
	}

	errs := errors.L()
	validAttrNames := []string{"description", "id", "name", "promote_from"}

	env := &Environment{
		Info: block.Range,
	}

	validAttrs := map[string]**ast.Attribute{
		"id":           &env.ID,
		"name":         &env.Name,
		"description":  &env.Description,
		"promote_from": &env.PromoteFrom,
	}

	for name, attr := range block.Attributes {
		if _, ok := validAttrs[name]; !ok {
			errs.Append(errors.E(
				ErrTerramateSchema,
				attr.Range,
				"valid attributes are [%s] but found %q",
				strings.Join(validAttrNames, ", "),
				name,
			))
			continue
		}
		target := validAttrs[name]
		*target = &attr
	}

	// Validate no sub-blocks
	for _, subBlock := range block.Blocks {
		errs.Append(errors.E(
			ErrTerramateSchema,
			subBlock.TypeRange,
			"unexpected block type %q in environment block",
			subBlock.Type,
		))
	}

	if err := errs.AsError(); err != nil {
		return err
	}

	p.ParsedConfig.Environments = append(p.ParsedConfig.Environments, env)
	return nil
}
