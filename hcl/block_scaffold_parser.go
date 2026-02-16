// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package hcl

import (
	"maps"
	"slices"
	"strings"

	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
)

// Scaffold block error constants.
const (
	ErrScaffoldInvalidLabels = ErrTerramateSchema + `: "bundle" block with invalid number of labels`
	ErrScaffoldInvalidBlock  = ErrTerramateSchema + `: "bundle" block contains invalid blocks`
)

// Scaffold is a `scaffold` block in the terramate-catalyst config.
type Scaffold struct {
	PackageSources *ast.Attribute
}

// ScaffoldBlockParser is a parser for the `scaffold` block.
type ScaffoldBlockParser struct{}

// Name returns the name of the block.
func (p *ScaffoldBlockParser) Name() string {
	return "scaffold"
}

// NewScaffoldBlockParser returns a new ScaffoldBlockParser.
func NewScaffoldBlockParser() *ScaffoldBlockParser {
	return &ScaffoldBlockParser{}
}

// Parse parses the "stack" block.
func (*ScaffoldBlockParser) Parse(p *TerramateParser, block *ast.Block) error {
	errs := errors.L()
	for _, block := range block.Body.Blocks {
		errs.Append(
			errors.E(ErrTerramateSchema, block.TypeRange, "unrecognized block %q", block.Type),
		)
	}

	var scaffoldCfg Scaffold

	validAttrs := map[string]**ast.Attribute{
		"package_sources": &scaffoldCfg.PackageSources,
	}

	for _, attr := range block.Attributes {
		if _, found := validAttrs[attr.Name]; !found {
			return errors.E(
				ErrTerramateSchema,
				attr.Range,
				"valid attributes are [%s] but found %q",
				strings.Join(slices.Sorted(maps.Keys(validAttrs)), ", "),
				attr.Name,
			)
		}
		err := setAttr(validAttrs[attr.Name], attr, "scaffold")
		if err != nil {
			return err
		}
	}

	p.ParsedConfig.Scaffold = &scaffoldCfg
	return nil
}
