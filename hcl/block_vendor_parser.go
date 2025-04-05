// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package hcl

import (
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/zclconf/go-cty/cty"
)

// VendorBlockParser is the parser for the "vendor" block.
type VendorBlockParser struct{}

// NewVendorBlockParser returns a new parser specification for the "vendor" block.
func NewVendorBlockParser() *VendorBlockParser {
	return &VendorBlockParser{}
}

// Name returns the type of the block.
func (*VendorBlockParser) Name() string {
	return "vendor"
}

// Parse parses the "vendor" block.
func (*VendorBlockParser) Parse(p *TerramateParser, block *ast.Block) (err error) {
	errs := errors.L()

	defer func() {
		if err != nil {
			err = errors.E(ErrTerramateSchema, err)
		}
	}()

	p.ParsedConfig.Vendor = &VendorConfig{}
	cfg := p.ParsedConfig.Vendor
	for _, attr := range block.Attributes {
		switch attr.Name {
		case "dir":
			attrVal, err := attr.Expr.Value(nil)
			if err != nil {
				errs.Append(errors.E(ErrTerramateSchema, err, attr.NameRange,
					"evaluating %s.%s", block.Type, attr.Name))
				continue
			}
			if attrVal.Type() != cty.String {
				errs.Append(errors.E(ErrTerramateSchema, attr.NameRange,
					"%s.%s must be string, got %s", block.Type, attr.Name, attrVal.Type,
				))
				continue
			}
			cfg.Dir = attrVal.AsString()
		default:
			errs.Append(errors.E(ErrTerramateSchema, attr.NameRange,
				"unrecognized attribute %s.%s", block.Type, attr.Name,
			))
		}
	}
	errs.Append(checkNoLabels(block))
	errs.Append(checkHasSubBlocks(block, "manifest"))

	if err := errs.AsError(); err != nil {
		return err
	}

	if len(block.Blocks) == 0 {
		return nil
	}

	manifestBlock := block.Blocks[0]

	errs.Append(checkNoAttributes(manifestBlock))
	errs.Append(checkNoLabels(manifestBlock))
	errs.Append(checkHasSubBlocks(manifestBlock, "default"))

	if err := errs.AsError(); err != nil {
		return err
	}

	cfg.Manifest = &ManifestConfig{}

	if len(manifestBlock.Blocks) == 0 {
		return nil
	}

	defaultBlock := manifestBlock.Blocks[0]

	errs.Append(checkNoBlocks(defaultBlock))

	cfg.Manifest.Default = &ManifestDesc{}

	for _, attr := range defaultBlock.Attributes {
		switch attr.Name {
		case "files":
			attrVal, err := attr.Expr.Value(nil)
			if err != nil {
				errs.Append(err)
				continue
			}
			if err := assignSet(attr.Attribute, &cfg.Manifest.Default.Files, attrVal); err != nil {
				errs.Append(errors.E(err, attr.NameRange))
			}
		default:
			errs.Append(errors.E(ErrTerramateSchema, attr.NameRange,
				"unrecognized attribute %s.%s", defaultBlock.Type, attr.Name,
			))
		}
	}

	return errs.AsError()
}
