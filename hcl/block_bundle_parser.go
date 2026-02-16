// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package hcl

import (
	"maps"

	hhcl "github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/hcl/v2/hclsyntax"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/info"
	"github.com/terramate-io/terramate/project"
	"github.com/zclconf/go-cty/cty"
)

// Bundle block error constants.
const (
	ErrBundleInvalidLabels          = ErrTerramateSchema + `: "bundle" block with invalid number of labels`
	ErrBundleInvalidBlock           = ErrTerramateSchema + `: "bundle" block contains invalid blocks`
	ErrBundleMissingSourceAttribute = ErrTerramateSchema + `: "bundle" block requires a "source" attribute`
)

// BundleTemplate is a `bundle` block in the terramate-catalyst config.
type BundleTemplate struct {
	Name       string
	UUID       *ast.Attribute
	Source     *ast.Attribute
	Workdir    project.Path
	InputsAttr *ast.Attribute
	Inputs     *ast.MergedBlock
	EnvValues  []*BundleEnvValues
	Info       info.Range
}

// BundleEnvValues holds environment-specific overrides for a bundle.
type BundleEnvValues struct {
	EnvID  *ast.Attribute
	Source *ast.Attribute
	Inputs *ast.MergedBlock
	Info   info.Range
}

// Bundle is a concrete, instantiated bundle derived from a BundleTemplate.
// It can no longer be instantiated directly; it always requires BundleTemplate.
type Bundle struct {
	Name        string
	Environment *ast.Attribute
	UUID        *ast.Attribute
	Source      *ast.Attribute
	Workdir     project.Path
	InputsAttr  *ast.Attribute
	Inputs      *ast.MergedBlock
	Info        info.Range
}

// BundleBlockParser is a parser for the `bundle` block.
type BundleBlockParser struct{}

// Name returns the name of the block.
func (p *BundleBlockParser) Name() string {
	return "bundle"
}

// NewBundleBlockParser returns a new BundleBlockParser.
func NewBundleBlockParser() *BundleBlockParser {
	return &BundleBlockParser{}
}

// Parse parses the `bundle` block.
func (*BundleBlockParser) Parse(p *TerramateParser, label ast.LabelBlockType, block *ast.MergedBlock) error {
	if label.NumLabels == 0 {
		return errors.E(
			ErrBundleInvalidLabels,
			block.RawOrigins[0].LabelRanges(),
			"expected 1 label but got %d",
			label.NumLabels,
		)
	}

	var (
		isNew     bool
		bundleTpl *BundleTemplate
	)

	name := label.Labels[0]

	for _, b := range p.ParsedConfig.Bundles {
		if b.Name == name {
			bundleTpl = b
			break
		}
	}

	if bundleTpl == nil {
		bundleTpl = &BundleTemplate{
			Name:    label.Labels[0],
			Workdir: block.RawOrigins[0].Range.Path().Dir(),
			Inputs:  ast.NewMergedBlock("inputs", []string{}),
			Info:    block.RawOrigins[0].Range,
		}
		isNew = true
	}

	switch label.NumLabels {
	case 1: // bundle "name"
		if err := parseBundleTemplate(bundleTpl, block, label); err != nil {
			return err
		}
	case 2: // bundle "name" "inputs" { ... }
		if err := parseInputsBlock(bundleTpl.Inputs, block, ast.NewEmptyLabelBlockType("inputs")); err != nil {
			return err
		}
	default:
		return errors.E(
			ErrBundleInvalidLabels,
			block.RawOrigins[0].LabelRanges(),
			"expected 1 or 2 labels but got %d",
			label.NumLabels,
		)
	}
	if isNew {
		p.ParsedConfig.Bundles = append(p.ParsedConfig.Bundles, bundleTpl)
	}
	return nil
}

func parseBundleTemplate(bundleTpl *BundleTemplate, block *ast.MergedBlock, label ast.LabelBlockType) error {
	if label.NumLabels != 1 {
		return errors.E(
			ErrTerramateSchema,
			block.RawOrigins[0].LabelRanges(),
			"bundle block must have exactly one label but got %d",
			label.NumLabels,
		)
	}

	validAttrs := map[string]**ast.Attribute{
		"uuid":   &bundleTpl.UUID,
		"source": &bundleTpl.Source,
		"inputs": &bundleTpl.InputsAttr,
	}
	err := parseBlockAttributes(block, validAttrs, ErrTerramateSchema)
	if err != nil {
		return err
	}

	if bundleTpl.Source == nil {
		return errors.E(
			ErrBundleMissingSourceAttribute,
			block.RawOrigins[0].TypeRange,
		)
	}

	for label, subBlock := range block.Blocks {
		switch subBlock.Type {
		case "inputs":
			err := parseInputsBlock(bundleTpl.Inputs, subBlock, label)
			if err != nil {
				return err
			}
		case "environment":
			err := parseBundleEnvValues(bundleTpl, subBlock, label)
			if err != nil {
				return err
			}
		default:
			return errors.E(
				ErrBundleInvalidBlock,
				block.RawOrigins[0].LabelRanges(),
				"unexpected block type %q",
				subBlock.Type,
			)
		}
	}

	return nil
}

func parseInputsBlock(inputs *ast.MergedBlock, block *ast.MergedBlock, label ast.LabelBlockType) error {
	if label.NumLabels != 0 {
		return errors.E(
			ErrTerramateSchema,
			block.RawOrigins[0].LabelRanges(),
			"inputs block must have no labels but got %d",
			label.NumLabels,
		)
	}
	maps.Copy(inputs.Attributes, block.Attributes)
	for _, subBlock := range block.Blocks {
		return errors.E(
			ErrBundleInvalidBlock,
			subBlock.RawOrigins[0].LabelRanges(),
			"unexpected block type %q",
			subBlock.Type,
		)
	}
	return nil
}

func parseBundleEnvValues(bundleTpl *BundleTemplate, block *ast.MergedBlock, label ast.LabelBlockType) error {
	if label.NumLabels != 1 {
		return errors.E(
			ErrTerramateSchema,
			block.RawOrigins[0].LabelRanges(),
			"environment block must have exactly one label but got %d",
			label.NumLabels,
		)
	}

	// This from a bundle template, where the syntax is `environment "name" { ... }`.
	// But it gets mapped to a normal bundle, where its just an attribute `environment = <name>`.
	// So we are turning the label into an attribute here.
	envIDAttr := ast.NewAttribute(block.RawOrigins[0].Range.HostPath(),
		&hhcl.Attribute{
			Name:  label.Labels[0],
			Expr:  &hclsyntax.LiteralValueExpr{Val: cty.StringVal(label.Labels[0])},
			Range: block.RawOrigins[0].LabelRanges(),
		})

	env := &BundleEnvValues{
		EnvID:  &envIDAttr,
		Inputs: ast.NewMergedBlock("inputs", []string{}),
		Info:   block.RawOrigins[0].Range,
	}

	validAttrs := map[string]**ast.Attribute{
		"source": &env.Source,
	}
	err := parseBlockAttributes(block, validAttrs, ErrTerramateSchema)
	if err != nil {
		return err
	}

	for subLabel, subBlock := range block.Blocks {
		switch subBlock.Type {
		case "inputs":
			err := parseInputsBlock(env.Inputs, subBlock, subLabel)
			if err != nil {
				return err
			}
		default:
			return errors.E(
				ErrBundleInvalidBlock,
				subBlock.RawOrigins[0].LabelRanges(),
				"unexpected block type %q in environment block",
				subBlock.Type,
			)
		}
	}

	bundleTpl.EnvValues = append(bundleTpl.EnvValues, env)
	return nil
}

// Validate validates all parsed bundle blocks.
func (*BundleBlockParser) Validate(p *TerramateParser) error {
	errs := errors.L()
	for _, bundleTpl := range p.ParsedConfig.Bundles {
		errs.Append(bundleTpl.Validate(p))
	}

	return errs.AsError()
}

// Validate validates the bundle template has all required attributes.
func (bundleTpl *BundleTemplate) Validate(p *TerramateParser) error {
	if bundleTpl.Source == nil {
		return errors.E(ErrTerramateSchema, "%s: bundle %q is missing required attribute %q",
			p.ParsedConfig.AbsDir(), bundleTpl.Name, "source")
	}
	return nil
}
