// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package hcl

import (
	"maps"
	"slices"
	"strings"

	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/info"
	"github.com/zclconf/go-cty/cty"
)

// Component block error constants.
const (
	ErrComponentMissingAttribute       = ErrTerramateSchema + `: unrecognized "component" attribute`
	ErrComponentMissingSourceAttribute = ErrTerramateSchema + `: "component" block requires a "source" attribute`
	ErrComponentAlreadyDeclared        = ErrTerramateSchema + `: "component" already declared`
	ErrComponentInputAlreadyDeclared   = ErrTerramateSchema + `: "component.inputs" attribute already declared`
)

// Component represents a parsed component instantiation.
type Component struct {
	Condition *ast.Attribute

	Name        string
	Environment *ast.Attribute
	Source      *ast.Attribute
	InputsAttr  *ast.Attribute
	Inputs      *ast.MergedBlock
	Info        info.Range

	// A component may be instantiated from within a bundle, in which case we track the bundle location.
	FromBundleSource string

	// In this case component comes from a bundle, this stores the values of the `bundle.` object.
	BundleObject *cty.Value
}

// ComponentBlockParser is a parser for component instantiation blocks.
type ComponentBlockParser struct{}

// Name returns the name of the block.
func (c *ComponentBlockParser) Name() string {
	return "component"
}

// NewComponentBlockParser creates a new instance of ComponentBlockParser.
func NewComponentBlockParser() *ComponentBlockParser {
	return &ComponentBlockParser{}
}

// Parse parses a "component" block.
func (c *ComponentBlockParser) Parse(p *TerramateParser, label ast.LabelBlockType, block *ast.MergedBlock) error {
	return c.parse(&p.ParsedConfig, label, block)
}

func (c *ComponentBlockParser) parse(parsed *Config, label ast.LabelBlockType, block *ast.MergedBlock) error {
	if label.NumLabels == 0 {
		return errors.E(
			ErrTerramateSchema,
			block.RawOrigins[0].LabelRanges(),
			"component block must have at least one label",
			len(block.Labels),
		)
	}

	if label.NumLabels > 2 {
		return errors.E(
			ErrTerramateSchema,
			block.RawOrigins[0].LabelRanges(),
			`the component block can be have either 1 or 2 labels, %d given`,
			label.NumLabels,
		)
	}

	var (
		isNew     bool
		component *Component
	)
	name := label.Labels[0]
	for _, c := range parsed.Components {
		if c.Name == name {
			component = c
			break
		}
	}

	if component == nil {
		isNew = true
		component = &Component{
			Name: name,

			// Inputs are populated from several sources.
			// - component.inputs block
			// - component."name".inputs block
			Inputs: ast.NewMergedBlock("inputs", []string{}),
			Info:   block.RawOrigins[0].Range,
		}
	}

	if label.NumLabels == 2 {
		if label.Labels[1] != "inputs" {
			return errors.E(
				ErrTerramateSchema,
				block.RawOrigins[0].LabelRanges(),
				"unexpected label %q, expected %q",
				label.Labels[1],
				"inputs",
			)
		}

		if err := c.parseInputsBlock(component, block); err != nil {
			return err
		}
	} else {
		if err := c.parseComponentInstantiationBlock(component, block); err != nil {
			return err
		}
	}

	if isNew {
		parsed.Components = append(parsed.Components, component)
	}
	return nil
}

func (c *ComponentBlockParser) parseComponentInstantiationBlock(component *Component, block *ast.MergedBlock) error {
	validAttrs := map[string]**ast.Attribute{
		"condition":   &component.Condition,
		"environment": &component.Environment,
		"source":      &component.Source,
		"inputs":      &component.InputsAttr,
	}

	for _, attr := range block.Attributes {
		if _, found := validAttrs[attr.Name]; !found {
			return errors.E(
				ErrComponentMissingAttribute,
				attr.Range,
				"valid attributes are [%s] but found %q",
				strings.Join(slices.Sorted(maps.Keys(validAttrs)), ", "),
				attr.Name,
			)
		}
		err := setAttr(validAttrs[attr.Name], attr, "component")
		if err != nil {
			return err
		}
	}
	if component.Source == nil {
		return errors.E(
			ErrComponentMissingSourceAttribute,
			block.RawOrigins[0].TypeRange,
		)
	}

	for label, block := range block.Blocks {
		if block.Type != "inputs" {
			return errors.E(
				ErrTerramateSchema,
				block.RawOrigins[0].LabelRanges(),
				"unexpected block type %q, expected %q",
				block.Type,
				"inputs",
			)
		}

		if label.NumLabels > 0 {
			return errors.E(
				ErrTerramateSchema,
				block.RawOrigins[0].LabelRanges(),
				`component.inputs block cannot have labels`,
			)
		}

		for name, attr := range block.Attributes {
			if other, found := component.Inputs.Attributes[name]; found {
				return errors.E(
					ErrComponentInputAlreadyDeclared,
					attr.Range,
					"input %q already declared at %s",
					name,
					other.Range.String(),
				)
			}
			component.Inputs.Attributes[name] = attr
		}

		// TODO(i4k): fail if sub blocks are present
	}

	return nil
}

func (c *ComponentBlockParser) parseInputsBlock(component *Component, block *ast.MergedBlock) error {
	for name, attr := range block.Attributes {
		if other, found := component.Inputs.Attributes[name]; found {
			return errors.E(
				ErrComponentInputAlreadyDeclared,
				attr.Range,
				"input %q already declared at %s",
				name,
				other.Range.String(),
			)
		}
		component.Inputs.Attributes[name] = attr
	}

	return nil
}

// Validate validates all parsed component blocks.
func (c *ComponentBlockParser) Validate(p *TerramateParser) error {
	errs := errors.L()
	for _, bundle := range p.ParsedConfig.Components {
		errs.Append(bundle.Validate(p))
	}

	return errs.AsError()
}

// Validate validates the component has all required attributes.
func (component *Component) Validate(p *TerramateParser) error {
	if component.Source == nil {
		return errors.E(ErrTerramateSchema, "%s: component %q is missing required attribute %q",
			p.ParsedConfig.AbsDir(), component.Name, "source")
	}
	return nil
}
