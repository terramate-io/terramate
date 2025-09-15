// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package hcl

import (
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/zclconf/go-cty/cty"
)

// TerramateBlockParser is the parser for the "terramate" block.
type TerramateBlockParser struct{}

// NewTerramateBlockParser returns a new parser specification for the "terramate" block.
func NewTerramateBlockParser() *TerramateBlockParser {
	return &TerramateBlockParser{}
}

// Name returns the type of the block.
func (*TerramateBlockParser) Name() string {
	return "terramate"
}

// Parse parses the "terramate" block.
func (*TerramateBlockParser) Parse(p *TerramateParser, block *ast.MergedBlock) error {
	tm := Terramate{}

	errKind := ErrTerramateSchema
	errs := errors.L()
	var foundReqVersion, foundAllowPrereleases bool
	for _, attr := range block.Attributes.SortedList() {
		value, diags := attr.Expr.Value(nil)
		if diags.HasErrors() {
			errs.Append(errors.E(errKind, diags))
		}
		switch attr.Name {
		case "required_version":
			if value.Type() != cty.String {
				errs.Append(errors.E(errKind, attr.Expr.Range(),
					"attribute is not a string"))

				continue
			}
			if foundReqVersion {
				errs.Append(errors.E(errKind, attr.NameRange,
					"duplicated attribute"))
			}
			foundReqVersion = true
			tm.RequiredVersion = value.AsString()

		case "required_version_allow_prereleases":
			if value.Type() != cty.Bool {
				errs.Append(errors.E(errKind, attr.Expr.Range(),
					"attribute is not a bool"))

				continue
			}

			if foundAllowPrereleases {
				errs.Append(errors.E(errKind, attr.NameRange,
					"duplicated attribute"))
			}

			foundAllowPrereleases = true
			tm.RequiredVersionAllowPreReleases = value.True()

		default:
			errs.Append(errors.E(errKind, attr.NameRange,
				"unsupported attribute %q", attr.Name))
		}
	}

	errs.AppendWrap(ErrTerramateSchema, block.ValidateSubBlocks("config"))

	configBlock, ok := block.Blocks[ast.NewEmptyLabelBlockType("config")]
	if ok {
		tm.Config = &RootConfig{}

		err := p.parseRootConfig(tm.Config, configBlock)
		if err != nil {
			errs.Append(errors.E(errKind, err))
		}
	}

	if err := errs.AsError(); err != nil {
		return err
	}
	p.ParsedConfig.Terramate = &tm
	return nil

}

// Validate postconditions after parsing.
func (*TerramateBlockParser) Validate(*TerramateParser) error {
	return nil
}
