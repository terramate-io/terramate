// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package hcl

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	hhcl "github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/printer"
)

// Define block error constants.
const (
	ErrUnrecognizedDefineBlock                      = ErrTerramateSchema + `: unrecognized "define" feature`
	ErrUnrecognizedDefineAttribute                  = ErrTerramateSchema + `: unrecognized "define" attribute`
	ErrUnrecognizedDefineSubBlock                   = ErrTerramateSchema + `: unrecognized "define" child block`
	ErrUnrecognizedDefineComponentAttribute         = ErrTerramateSchema + `: unrecognized "define.component" attribute`
	ErrUnrecognizedMetadataBlock                    = ErrTerramateSchema + `: unrecognized "define.component.metadata" feature`
	ErrUnrecognizedComponentMetadataAttribute       = ErrTerramateSchema + `: unrecognized "define.component.metadata" attribute`
	ErrUnrecognizedDefineBundleAttribute            = ErrTerramateSchema + `: unrecognized "define.bundle" attribute`
	ErrUnrecognizedStackAttribute                   = ErrTerramateSchema + `: unrecognized "define.bundle.stack" attribute`
	ErrUnrecognizedStackMetadataAttribute           = ErrTerramateSchema + `: unrecognized "define.bundle.stack.<label>.metadata" attribute`
	ErrUnrecognizedDefineBundleScaffoldingAttribute = ErrTerramateSchema + `: unrecognized "define.bundle.scaffolding" attribute`
	ErrUnrecognizedDefineEnvironmentsAttribute      = ErrTerramateSchema + `: unrecognized "environments" attribute`
	ErrUnrecognizedInputBlock                       = ErrTerramateSchema + `: unrecognized "define.component.input" feature`
	ErrUnrecognizedInputAttribute                   = ErrTerramateSchema + `: unrecognized "define.component.input" attribute`
	ErrUnrecognizedExportBlock                      = ErrTerramateSchema + `: unrecognized "define.component.export" feature`
	ErrUnrecognizedExportAttribute                  = ErrTerramateSchema + `: unrecognized "define.component.export" attribute`
	ErrUnrecognizedSchemaAttribute                  = ErrTerramateSchema + `: unrecognized "define.schema" attribute`
	ErrUnrecognizedUsesAttribute                    = ErrTerramateSchema + `: unrecognized "uses" attribute`
)

// ErrMissingAttribute generates errors for a missing attribute.
func ErrMissingAttribute(blockName, attrName string) errors.Kind {
	return ErrTerramateSchema +
		errors.Kind(fmt.Sprintf(`: %q block requires a %q attribute`, blockName, attrName))
}

// Define represents the parsed "define" block in
type Define struct {
	Component *DefineComponent
	Bundle    *DefineBundle
	Schemas   []*DefineSchema
}

// DefineComponent represents a component defined in the "define" block.
type DefineComponent struct {
	Metadata    Metadata
	Inputs      map[string]*DefineInput
	UsesSchemas []*UsesSchemas
}

// DefineBundle represents a bundle defined in the "define" block.
type DefineBundle struct {
	Alias *ast.Attribute

	Metadata     Metadata
	Stacks       map[string]*DefineStack
	Inputs       map[string]*DefineInput
	Exports      map[string]*DefineExport
	Scaffolding  DefineBundleScaffolding
	UsesSchemas  []*UsesSchemas
	Environments DefineEnvironmentsOptions
}

// DefineSchema represents a schema defined in the "define" block.
type DefineSchema struct {
	Name             string
	Description      *ast.Attribute
	Type             *ast.Attribute
	ObjectAttributes []*DefineObjectAttribute
	DefRange         hhcl.Range
}

// DefineStack represents a stack defined in the "define bundle" block.
type DefineStack struct {
	Condition  *ast.Attribute
	Metadata   StackMetadata
	Components []*Component
}

// DefineBundleScaffolding represents scaffolding parameters of a bundle.
type DefineBundleScaffolding struct {
	Enabled []*Condition
	Name    *ast.Attribute
	Path    *ast.Attribute
}

// DefineEnvironmentsOptions represents environment parameters of a bundle or component.
type DefineEnvironmentsOptions struct {
	Required *ast.Attribute
}

// Condition is a condition block.
type Condition struct {
	Condition    *ast.Attribute
	ErrorMessage *ast.Attribute
}

// StackMetadata represents the metadata of a stack.
type StackMetadata struct {
	Path *ast.Attribute

	Name        *ast.Attribute
	Description *ast.Attribute
	Tags        *ast.Attribute
	After       *ast.Attribute
	Before      *ast.Attribute
	Wants       *ast.Attribute
	WantedBy    *ast.Attribute
	Watch       *ast.Attribute
}

// Metadata represents the metadata of components/bundles.
type Metadata struct {
	Class        *ast.Attribute
	Name         *ast.Attribute
	Description  *ast.Attribute
	Technologies *ast.Attribute
	Version      *ast.Attribute
}

// UsesSchemas represents a "uses schemas" block.
type UsesSchemas struct {
	Name     string
	Source   *ast.Attribute
	DefRange hhcl.Range
}

// DefineInput represents an input for a component/bundle.
type DefineInput struct {
	Name                string
	Prompt              *ast.Attribute
	Description         *ast.Attribute
	Type                *ast.Attribute
	ObjectAttributes    []*DefineObjectAttribute
	Default             *ast.Attribute
	Options             *ast.Attribute
	Multiline           *ast.Attribute
	Multiselect         *ast.Attribute
	RequiredForScaffold *ast.Attribute
	DefRange            hhcl.Range
}

// DefineObjectAttribute represents an object attribute of a top-level schema definition.
type DefineObjectAttribute struct {
	Name        string
	Description *ast.Attribute
	Type        *ast.Attribute
	Default     *ast.Attribute
	Required    *ast.Attribute
	DefRange    hhcl.Range
}

// DefineExport represents an export for a bundle.
type DefineExport struct {
	Name        string
	Description *ast.Attribute
	Value       *ast.Attribute
	DefRange    hhcl.Range
}

// DefineBlockParser is a parser for the "define" block in
type DefineBlockParser struct{}

// NewDefineBlockParser creates a new instance of DefineBlockParser.
func NewDefineBlockParser() *DefineBlockParser {
	return &DefineBlockParser{}
}

// Name returns the name of the block.
func (d *DefineBlockParser) Name() string {
	return "define"
}

func newDefine(cfg *Config) *Define {
	var define *Define
	for _, d := range cfg.Defines {
		if d.Component != nil || d.Bundle != nil || len(d.Schemas) > 0 {
			d := d
			define = d
			break
		}
	}
	if define == nil {
		define = &Define{}
		cfg.Defines = append(cfg.Defines, define)
	}
	return define
}

func newDefineComponent() *DefineComponent {
	return &DefineComponent{
		Inputs: make(map[string]*DefineInput),
	}
}

func newDefineBundle() *DefineBundle {
	return &DefineBundle{
		Stacks:  make(map[string]*DefineStack),
		Inputs:  make(map[string]*DefineInput),
		Exports: make(map[string]*DefineExport),
	}
}

// Parse parses the "define" block.
func (d *DefineBlockParser) Parse(p *TerramateParser, label ast.LabelBlockType, block *ast.MergedBlock) error {
	switch label.NumLabels {
	case 0:
		err := parseUnlabeledDefineBlock(&p.ParsedConfig, block)
		if err != nil {
			return err
		}
	case 1:
		if label.Labels[0] != "component" && label.Labels[0] != "bundle" {
			return errors.E(
				ErrUnrecognizedDefineBlock,
				block.RawOrigins[0].LabelRanges(),
				`unexpected label %q, expected "component" or "bundle"`,
				label.Labels[0],
			)
		}
		define := newDefine(&p.ParsedConfig)
		if label.Labels[0] == "component" {
			if define.Component == nil {
				define.Component = newDefineComponent()
			}
			err := parseDefineComponentBlock(ast.NewEmptyLabelBlockType("component"), block, define.Component)
			if err != nil {
				return err
			}
		} else {
			if define.Bundle == nil {
				define.Bundle = newDefineBundle()
			}
			err := parseDefineBundleBlock(ast.NewEmptyLabelBlockType("bundle"), block, define.Bundle)
			if err != nil {
				return err
			}
		}
	case 2:
		define := newDefine(&p.ParsedConfig)

		switch label.Labels[0] {
		case "component":
			if define.Component == nil {
				define.Component = newDefineComponent()
			}

			switch label.Labels[1] {
			case "metadata":
				err := parseMetadataBlock(block, &define.Component.Metadata)
				if err != nil {
					return err
				}

			default:
				return errors.E(
					ErrUnrecognizedDefineSubBlock,
					block.RawOrigins[0].LabelRanges(),
					`unexpected label %q, expected "metadata" or "environments"`,
					label.Labels[1],
				)
			}

		case "bundle":
			if define.Bundle == nil {
				define.Bundle = newDefineBundle()
			}

			switch label.Labels[1] {
			case "metadata":
				err := parseMetadataBlock(block, &define.Bundle.Metadata)
				if err != nil {
					return err
				}

			case "scaffolding":
				if err := parseDefineBundleScaffoldingBlock(block, &define.Bundle.Scaffolding); err != nil {
					return err
				}

			case "environments":
				if err := parseDefineEnvironmentsBlock(block, &define.Bundle.Environments); err != nil {
					return err
				}

			default:
				return errors.E(
					ErrUnrecognizedDefineSubBlock,
					block.RawOrigins[0].LabelRanges(),
					`unexpected label %q, expected "metadata", "scaffolding" or "environments"`,
					label.Labels[1],
				)
			}

		case "schema":
			schema, err := parseDefineSchemaBlock(label, block)
			if err != nil {
				return err
			}
			define.Schemas = append(define.Schemas, schema)

		default:
			return errors.E(
				ErrUnrecognizedDefineBlock,
				block.RawOrigins[0].LabelRanges(),
				`unexpected label %q, expected "component", "bundle" or "schema"`,
				label.Labels[0],
			)
		}

	case 3:
		switch label.Labels[0] {
		case "component":
			switch label.Labels[1] {
			case "input":
				// OK
			default:
				return errors.E(
					ErrUnrecognizedDefineSubBlock,
					block.RawOrigins[0].LabelRanges(),
					`unexpected label %q, expected "input"`,
					label.Labels[1],
				)
			}

		case "bundle":
			switch label.Labels[1] {
			case "input", "export", "stack":
				// OK
			default:
				return errors.E(
					ErrUnrecognizedDefineSubBlock,
					block.RawOrigins[0].LabelRanges(),
					`unexpected label %q, expected "input", "export" or "stack"`,
					label.Labels[1],
				)
			}

		default:
			return errors.E(
				ErrUnrecognizedDefineBlock,
				block.RawOrigins[0].LabelRanges(),
				`unexpected label %q, expected "component" or "bundle"`,
				label.Labels[0],
			)
		}

		define := newDefine(&p.ParsedConfig)

		switch label.Labels[1] {
		case "input":
			// handles: define component input
			// handles: define bundle input
			var inputs map[string]*DefineInput
			if label.Labels[0] == "component" {
				if define.Component == nil {
					define.Component = newDefineComponent()
				}
				inputs = define.Component.Inputs
			} else {
				if define.Bundle == nil {
					define.Bundle = newDefineBundle()
				}
				inputs = define.Bundle.Inputs
			}
			name := label.Labels[2]
			input, found := inputs[name]
			if !found {
				input = &DefineInput{
					Name:     name,
					DefRange: block.RawOrigins[0].LabelRanges(),
				}
				inputs[name] = input
			}
			err := parseInputBody(block, input)
			if err != nil {
				return err
			}

		case "export":
			// handles: define bundle export
			if define.Bundle == nil {
				define.Bundle = newDefineBundle()
			}
			exports := define.Bundle.Exports

			name := label.Labels[2]
			export, found := exports[name]
			if !found {
				export = &DefineExport{
					Name:     name,
					DefRange: block.RawOrigins[0].LabelRanges(),
				}
				exports[name] = export
			}
			err := parseExportBody(block, export)
			if err != nil {
				return err
			}
		case "stack":
			// handles: define bundle stack "id"
			if define.Bundle == nil {
				define.Bundle = newDefineBundle()
			}

			lb, err := ast.NewLabelBlockType("stack", block.Labels[2:])
			if err != nil {
				return err
			}
			err = parseDefineBundleStackBlock(define.Bundle.Stacks, block, lb)
			if err != nil {
				return err
			}
		}
	case 4:
		// only define.bundle.stack.<label>.metadata implemented here
		if label.Labels[0] != "bundle" {
			return errors.E(
				ErrUnrecognizedDefineBlock,
				block.RawOrigins[0].LabelRanges(),
				`with this amount of labels, only "define bundle stack <label> metadata" is supported`,
			)
		}
		if label.Labels[1] != "stack" {
			return errors.E(
				ErrUnrecognizedDefineSubBlock,
				block.RawOrigins[0].LabelRanges(),
				`unexpected label %q, expected "stack"`,
				label.Labels[1],
			)
		}

		stackName := label.Labels[2]
		if label.Labels[3] != "metadata" {
			return errors.E(
				ErrUnrecognizedDefineSubBlock,
				block.RawOrigins[0].LabelRanges(),
				`unexpected label %q, expected "metadata"`,
				label.Labels[3],
			)
		}
		define := newDefine(&p.ParsedConfig)
		// handles: define bundle stack "name"
		if define.Bundle == nil {
			define.Bundle = newDefineBundle()
		}
		lb, err := ast.NewLabelBlockType("stack", []string{stackName, "metadata"})
		if err != nil {
			return err
		}
		err = parseDefineBundleStackBlock(define.Bundle.Stacks, block, lb)
		if err != nil {
			return err
		}

	default:
		return errors.E(
			ErrTerramateSchema,
			block.RawOrigins[0].LabelRanges(),
			"unexpected number of labels in define block",
		)
	}
	return nil
}

func parseUnlabeledDefineBlock(cfg *Config, block *ast.MergedBlock) error {
	if block.Type != "define" {
		panic(errors.E(errors.ErrInternal, "unexpected block type %q, expected 'define'", block.Type))
	}

	if len(block.Labels) > 0 {
		panic(errors.E(errors.ErrInternal, "unexpected block labels %q, expected no labels", block.Labels))
	}

	errs := errors.L()
	for _, attr := range block.Attributes {
		errs.Append(errors.E(
			ErrUnrecognizedDefineAttribute,
			attr.Range,
			`define block must not have attributes`,
		))
	}
	if err := errs.AsError(); err != nil {
		return err
	}
	var define *Define
	for label, childBlock := range block.Blocks {
		if childBlock.Type != "component" && childBlock.Type != "bundle" && childBlock.Type != "schema" {
			return errors.E(
				ErrUnrecognizedDefineSubBlock,
				childBlock.RawOrigins[0].DefRange(),
				`unexpected block type %q, expected "component", "bundle" or "schema"`,
				childBlock.Type,
			)
		}

		if define == nil {
			define = newDefine(cfg)
		}
		switch childBlock.Type {
		case "component":
			if define.Component == nil {
				define.Component = newDefineComponent()
			}
			if err := parseDefineComponentBlock(label, childBlock, define.Component); err != nil {
				return err
			}
		case "bundle":
			if define.Bundle == nil {
				define.Bundle = newDefineBundle()
			}
			if err := parseDefineBundleBlock(label, childBlock, define.Bundle); err != nil {
				return err
			}
		case "schema":
			if label.NumLabels != 1 {
				return errors.E(
					childBlock.RawOrigins[0].LabelRanges(),
					`schema block must have exactly one label`,
				)
			}
			schema, err := parseDefineSchemaBlock(label, childBlock)
			if err != nil {
				return err
			}
			define.Schemas = append(define.Schemas, schema)
		}
	}
	return nil
}

func parseDefineComponentBlock(label ast.LabelBlockType, block *ast.MergedBlock, ret *DefineComponent) error {
	switch label.NumLabels {
	case 0:
		for blockLabels, subBlock := range block.Blocks {
			switch subBlock.Type {
			case "metadata":
				if err := parseMetadataBlock(subBlock, &ret.Metadata); err != nil {
					return err
				}
			case "input":
				if err := parseInputBlock(ret.Inputs, subBlock, blockLabels); err != nil {
					return err
				}
			case "uses":
				if err := parseUsesSchemasBlock(&ret.UsesSchemas, subBlock, blockLabels); err != nil {
					return err
				}

			default:
				return errors.E(
					ErrUnrecognizedDefineSubBlock,
					subBlock.RawOrigins[0].DefRange(),
					`unexpected block type %q, expected "metadata", "input", "uses schemas" or "environments"`,
					subBlock.Type,
				)
			}
		}
	case 1:
		switch label.Labels[0] {
		case "metadata":
			if err := parseMetadataBlock(block, &ret.Metadata); err != nil {
				return err
			}
		case "input":
			return errors.E(
				block.RawOrigins[0].DefRange(),
				`component block with "input" label must have a user-supplied "name" label`,
			)
		default:
			return errors.E(
				block.RawOrigins[0].DefRange(),
				"unexpected block type %q, expected 'metadata', 'input' or 'environments'",
				block.Type,
			)
		}
	case 2:
		switch label.Labels[0] {
		case "input":
			lb, err := ast.NewLabelBlockType("input", label.Labels[1:])
			if err != nil {
				return err
			}
			if err := parseInputBlock(ret.Inputs, block, lb); err != nil {
				return err
			}
		default:
			return errors.E(
				block.RawOrigins[0].DefRange(),
				"unexpected block type %q, expected 'input'",
				block.Type,
			)
		}
	default:
		return errors.E(
			block.RawOrigins[0].DefRange(),
			"unexpected number of labels in component block",
		)
	}
	return nil
}

func parseDefineBundleBlock(label ast.LabelBlockType, block *ast.MergedBlock, ret *DefineBundle) error {
	switch label.NumLabels {
	case 0:
		validAttrs := map[string]**ast.Attribute{
			"alias": &ret.Alias,
		}
		if err := parseBlockAttributes(block, validAttrs, ErrUnrecognizedDefineBundleAttribute); err != nil {
			return err
		}

		for labels, subBlock := range block.Blocks {
			switch subBlock.Type {
			case "metadata":
				if labels.NumLabels != 0 {
					return errors.E(
						subBlock.RawOrigins[0].LabelRanges(),
						`unexpected label %q, expected no labels`,
						labels.Labels[0],
					)
				}
				if err := parseMetadataBlock(subBlock, &ret.Metadata); err != nil {
					return err
				}
			case "stack":
				if err := parseDefineBundleStackBlock(ret.Stacks, subBlock, labels); err != nil {
					return err
				}
			case "input":
				if err := parseInputBlock(ret.Inputs, subBlock, labels); err != nil {
					return err
				}
			case "export":
				if err := parseDefineBundleExportBlock(ret.Exports, subBlock, labels); err != nil {
					return err
				}
			case "scaffolding":
				if labels.NumLabels != 0 {
					return errors.E(
						subBlock.RawOrigins[0].LabelRanges(),
						`unexpected label %q, expected no labels`,
						labels.Labels[0],
					)
				}
				if err := parseDefineBundleScaffoldingBlock(subBlock, &ret.Scaffolding); err != nil {
					return err
				}
			case "uses":
				if err := parseUsesSchemasBlock(&ret.UsesSchemas, subBlock, labels); err != nil {
					return err
				}
			case "environments":
				if labels.NumLabels != 0 {
					return errors.E(
						subBlock.RawOrigins[0].LabelRanges(),
						`unexpected label %q, expected no labels`,
						labels.Labels[0],
					)
				}
				if err := parseDefineEnvironmentsBlock(subBlock, &ret.Environments); err != nil {
					return err
				}
			default:
				return errors.E(
					ErrUnrecognizedDefineSubBlock,
					subBlock.RawOrigins[0].DefRange(),
					`unexpected block type %q, expected "metadata", "stack", "input", "export", "scaffolding", "environments" or "uses schemas"`,
					subBlock.Type,
				)
			}
		}
	case 1:
		switch label.Labels[0] {
		case "metadata":
			if err := parseMetadataBlock(block, &ret.Metadata); err != nil {
				return err
			}
		case "scaffolding":
			if err := parseDefineBundleScaffoldingBlock(block, &ret.Scaffolding); err != nil {
				return err
			}
		case "environments":
			if err := parseDefineEnvironmentsBlock(block, &ret.Environments); err != nil {
				return err
			}
		case "stack":
			return errors.E(
				block.RawOrigins[0].DefRange(),
				`bundle block with "stack" label must have a user-supplied "name" label`,
			)
		case "input":
			return errors.E(
				block.RawOrigins[0].DefRange(),
				`bundle block with "input" label must have a user-supplied "name" label`,
			)
		case "export":
			return errors.E(
				block.RawOrigins[0].DefRange(),
				`bundle block with "export" label must have a user-supplied "name" label`,
			)
		default:
			return errors.E(
				block.RawOrigins[0].DefRange(),
				"unexpected block type %q, expected 'metadata', 'input', 'export', 'stack', 'scaffolding' or 'environments'",
				block.Type,
			)
		}
	case 2:
		switch label.Labels[0] {
		case "input":
			lb, err := ast.NewLabelBlockType("input", label.Labels[1:])
			if err != nil {
				return err
			}
			if err := parseInputBlock(ret.Inputs, block, lb); err != nil {
				return err
			}
		case "export":
			lb, err := ast.NewLabelBlockType("export", label.Labels[1:])
			if err != nil {
				return err
			}
			if err := parseDefineBundleExportBlock(ret.Exports, block, lb); err != nil {
				return err
			}
		case "stack":
			lb, err := ast.NewLabelBlockType("stack", label.Labels[1:])
			if err != nil {
				return err
			}
			if err := parseDefineBundleStackBlock(ret.Stacks, block, lb); err != nil {
				return err
			}
		default:
			return errors.E(
				block.RawOrigins[0].DefRange(),
				"unexpected block type %q, expected 'metadata', 'input', or 'export'",
				block.Type,
			)
		}
	default:
		return errors.E(
			block.RawOrigins[0].DefRange(),
			"unexpected number of labels in bundle block",
		)
	}
	return nil
}

func parseBlockAttributes(block *ast.MergedBlock, validAttrs map[string]**ast.Attribute, errKind errors.Kind) error {
	errs := errors.L()
	for _, attr := range block.Attributes {
		if _, ok := validAttrs[attr.Name]; !ok {
			errs.Append(errors.E(
				errKind,
				attr.Range,
				"valid attributes are [%s] but found %q",
				strings.Join(slices.Sorted(maps.Keys(validAttrs)), ", "),
				attr.Name,
			))
			continue
		}

		if err := setAttr(validAttrs[attr.Name], attr, block.Type); err != nil {
			return err
		}
	}
	if err := errs.AsError(); err != nil {
		return err
	}
	return nil
}

func parseMetadataBlock(block *ast.MergedBlock, ret *Metadata) error {
	errs := errors.L()
	for _, subBlock := range block.Blocks {
		errs.Append(errors.E(
			ErrUnrecognizedMetadataBlock,
			subBlock.RawOrigins[0].DefRange(),
			`unexpected block type %q, metadata block must not have sub-blocks`,
			subBlock.Type,
		))
	}
	if err := errs.AsError(); err != nil {
		return err
	}
	validAttrs := map[string]**ast.Attribute{
		"class":        &ret.Class,
		"name":         &ret.Name,
		"description":  &ret.Description,
		"technologies": &ret.Technologies,
		"version":      &ret.Version,
	}
	if err := parseBlockAttributes(block, validAttrs, ErrUnrecognizedComponentMetadataAttribute); err != nil {
		return err
	}
	if ret.Class == nil {
		return errors.E(ErrMissingAttribute("metadata", "class"), block.RawOrigins[0].TypeRange)
	}
	if ret.Name == nil {
		return errors.E(ErrMissingAttribute("metadata", "name"), block.RawOrigins[0].TypeRange)
	}
	if ret.Version == nil {
		return errors.E(ErrMissingAttribute("metadata", "version"), block.RawOrigins[0].TypeRange)
	}
	return nil
}

func setAttr(target **ast.Attribute, newAttr ast.Attribute, blockName ast.BlockType) error {
	if *target != nil {
		return errors.E(
			ErrTerramateSchema,
			newAttr.Range,
			"duplicate attribute %q in %s block (first defined at %s)",
			newAttr.Name,
			string(blockName),
			(*target).Range.String(),
		)
	}

	*target = &newAttr
	return nil
}

func parseInputBlock(inputs map[string]*DefineInput, block *ast.MergedBlock, label ast.LabelBlockType) error {
	if block.Type != "input" {
		return errors.E(
			ErrUnrecognizedDefineSubBlock,
			block.RawOrigins[0].DefRange(),
			`unexpected block type %q, expected "input"`,
			block.Type,
		)
	}

	if label.NumLabels == 0 {
		return errors.E(
			block.RawOrigins[0].DefRange(),
			"input block must have a label",
		)
	}
	if label.NumLabels > 1 {
		return errors.E(
			block.RawOrigins[0].DefRange(),
			"input block must have only one label",
		)
	}

	name := label.Labels[0]
	input, found := inputs[name]
	if !found {
		input = &DefineInput{
			Name:     name,
			DefRange: block.RawOrigins[0].DefRange(),
		}
		inputs[name] = input
	}
	return parseInputBody(block, input)
}

func parseInputBody(block *ast.MergedBlock, ret *DefineInput) error {
	validAttrs := map[string]**ast.Attribute{
		"prompt":                &ret.Prompt,
		"description":           &ret.Description,
		"type":                  &ret.Type,
		"options":               &ret.Options,
		"multiline":             &ret.Multiline,
		"multiselect":           &ret.Multiselect,
		"required_for_scaffold": &ret.RequiredForScaffold,
		"default":               &ret.Default,
	}
	for _, attr := range block.Attributes {
		if _, ok := validAttrs[attr.Name]; !ok {
			return errors.E(
				ErrUnrecognizedInputAttribute,
				attr.Range,
				"valid attributes are [%s] but found %q",
				strings.Join(slices.Sorted(maps.Keys(validAttrs)), ", "),
				attr.Name,
			)
		}
		attr := attr
		if err := setAttr(validAttrs[attr.Name], attr, block.Type); err != nil {
			return err
		}
	}
	if ret.RequiredForScaffold != nil {
		printer.Stderr.Warn(
			errors.E(ret.RequiredForScaffold.Range, "attribute 'required_for_scaffold' is deprecated. it no longer has any effect."),
		)
	}

	for labels, subBlock := range block.Blocks {
		switch subBlock.Type {
		case "attribute":
			typeAttr, err := parseDefineObjectAttributeBlock(labels, subBlock)
			if err != nil {
				return err
			}
			ret.ObjectAttributes = append(ret.ObjectAttributes, typeAttr)
		default:
			return errors.E(
				ErrUnrecognizedDefineSubBlock,
				subBlock.RawOrigins[0].DefRange(),
				`unexpected block type %q, expected "attribute"`,
				subBlock.Type,
			)
		}
	}

	return nil
}

func parseDefineBundleExportBlock(exports map[string]*DefineExport, block *ast.MergedBlock, label ast.LabelBlockType) error {
	if block.Type != "export" {
		return errors.E(
			ErrUnrecognizedDefineSubBlock,
			block.RawOrigins[0].DefRange(),
			`unexpected block type %q, expected "export"`,
			block.Type,
		)
	}

	if label.NumLabels == 0 {
		return errors.E(
			block.RawOrigins[0].DefRange(),
			"output block must have a label",
		)
	}
	if label.NumLabels > 1 {
		return errors.E(
			block.RawOrigins[0].DefRange(),
			"output block must have only one label",
		)
	}
	errs := errors.L()
	for _, subBlock := range block.Blocks {
		errs.Append(errors.E(
			ErrUnrecognizedExportBlock,
			subBlock.RawOrigins[0].DefRange(),
			`unexpected block type %q, export block must not have sub-blocks`,
			subBlock.Type,
		))
	}
	if err := errs.AsError(); err != nil {
		return err
	}
	name := label.Labels[0]
	output, found := exports[name]
	if !found {
		output = &DefineExport{
			Name:     name,
			DefRange: block.RawOrigins[0].DefRange(),
		}
		exports[name] = output
	}
	return parseExportBody(block, output)
}

func parseExportBody(block *ast.MergedBlock, ret *DefineExport) error {
	validAttrs := map[string]**ast.Attribute{
		"description": &ret.Description,
		"value":       &ret.Value,
	}
	for _, attr := range block.Attributes {
		if _, ok := validAttrs[attr.Name]; !ok {
			return errors.E(
				ErrUnrecognizedExportAttribute,
				attr.Range,
				"valid attributes are [%s] but found %q",
				strings.Join(slices.Sorted(maps.Keys(validAttrs)), ", "),
				attr.Name,
			)
		}
		attr := attr
		if err := setAttr(validAttrs[attr.Name], attr, block.Type); err != nil {
			return err
		}
	}
	return nil
}

func parseDefineBundleStackBlock(stacks map[string]*DefineStack, block *ast.MergedBlock, label ast.LabelBlockType) error {
	if label.NumLabels == 0 {
		return errors.E(
			block.RawOrigins[0].DefRange(),
			"stack block must have a label",
		)
	}

	name := label.Labels[0]
	stack, found := stacks[name]
	if !found {
		stack = &DefineStack{}
		stacks[name] = stack
	}

	errs := errors.L()
	switch label.NumLabels {
	case 1:
		validAttrs := map[string]**ast.Attribute{
			"condition": &stack.Condition,
		}
		errs.Append(parseBlockAttributes(block, validAttrs, ErrUnrecognizedStackAttribute))

		for label, subBlock := range block.Blocks {
			switch subBlock.Type {
			case "metadata":
				if label.NumLabels != 0 {
					return errors.E(
						subBlock.RawOrigins[0].LabelRanges(),
						`unexpected label %q, expected no labels`,
						label.Labels[0],
					)
				}

				validAttrs := map[string]**ast.Attribute{
					"path":        &stack.Metadata.Path,
					"name":        &stack.Metadata.Name,
					"description": &stack.Metadata.Description,
					"tags":        &stack.Metadata.Tags,
					"after":       &stack.Metadata.After,
					"before":      &stack.Metadata.Before,
					"wants":       &stack.Metadata.Wants,
					"wanted_by":   &stack.Metadata.WantedBy,
					"watch":       &stack.Metadata.Watch,
				}
				errs.Append(parseBlockAttributes(subBlock, validAttrs, ErrUnrecognizedStackMetadataAttribute))

			case "component":
				tempCfg := Config{}
				componentParser := NewComponentBlockParser()
				err := componentParser.parse(&tempCfg, label, subBlock)
				if err != nil {
					errs.Append(err)
					continue
				}

				if len(tempCfg.Components) != 1 {
					panic(errors.E(errors.ErrInternal, "recursive parser got unexpected number of components: %d", len(tempCfg.Components)))
				}
				stack.Components = append(stack.Components, tempCfg.Components[0])
			default:
				errs.Append(errors.E(
					ErrUnrecognizedDefineSubBlock,
					subBlock.RawOrigins[0].DefRange(),
					`unexpected block %q, expected "metadata" or "component"`,
					subBlock.Type,
				))
			}
		}
	case 2:
		if label.Labels[1] != "metadata" {
			return errors.E(
				ErrUnrecognizedDefineSubBlock,
				block.RawOrigins[0].LabelRanges(),
				`unexpected label %q, expected "metadata"`,
				label.Labels[1],
			)
		}
		errs := errors.L()
		validAttrs := map[string]**ast.Attribute{
			"path":        &stack.Metadata.Path,
			"name":        &stack.Metadata.Name,
			"description": &stack.Metadata.Description,
			"tags":        &stack.Metadata.Tags,
			"after":       &stack.Metadata.After,
			"before":      &stack.Metadata.Before,
			"wants":       &stack.Metadata.Wants,
			"wanted_by":   &stack.Metadata.WantedBy,
			"watch":       &stack.Metadata.Watch,
		}
		errs.Append(parseBlockAttributes(block, validAttrs, ErrUnrecognizedStackMetadataAttribute))
	default:
		return errors.E(
			block.RawOrigins[0].DefRange(),
			"unexpected number of labels in stack block",
		)
	}
	return errs.AsError()
}

func parseDefineBundleScaffoldingBlock(block *ast.MergedBlock, ret *DefineBundleScaffolding) error {
	validAttrs := map[string]**ast.Attribute{
		"path": &ret.Path,
		"name": &ret.Name,
	}
	if err := parseBlockAttributes(block, validAttrs, ErrUnrecognizedDefineBundleScaffoldingAttribute); err != nil {
		return err
	}

	errs := errors.L()
	for labels, subBlock := range block.Blocks {
		switch subBlock.Type {
		case "enabled":
			errs.Append(parseConditionBlock(&ret.Enabled, subBlock, labels))
		default:
			errs.Append(errors.E(
				ErrUnrecognizedDefineSubBlock,
				subBlock.RawOrigins[0].DefRange(),
				`unexpected block type %q, expected "enabled"`,
				subBlock.Type,
			))
		}
	}
	if err := errs.AsError(); err != nil {
		return err
	}
	return nil
}

func parseDefineEnvironmentsBlock(block *ast.MergedBlock, ret *DefineEnvironmentsOptions) error {
	validAttrs := map[string]**ast.Attribute{
		"required": &ret.Required,
	}
	if err := parseBlockAttributes(block, validAttrs, ErrUnrecognizedDefineEnvironmentsAttribute); err != nil {
		return err
	}

	errs := errors.L()
	for _, subBlock := range block.Blocks {
		errs.Append(errors.E(
			ErrUnrecognizedDefineSubBlock,
			subBlock.RawOrigins[0].DefRange(),
			`unexpected block type %q, environments block must not have sub-blocks`,
			subBlock.Type,
		))
	}
	if err := errs.AsError(); err != nil {
		return err
	}
	return nil
}

func parseConditionBlock(conditions *[]*Condition, block *ast.MergedBlock, _ ast.LabelBlockType) error {
	errs := errors.L()
	for _, subBlock := range block.Blocks {
		errs.Append(errors.E(
			ErrUnrecognizedDefineBundleScaffoldingAttribute,
			subBlock.RawOrigins[0].DefRange(),
			`unexpected block type %q, block must not have sub-blocks`,
			subBlock.Type,
		))
	}
	if err := errs.AsError(); err != nil {
		return err
	}

	r := Condition{}

	validAttrs := map[string]**ast.Attribute{
		"condition":     &r.Condition,
		"error_message": &r.ErrorMessage,
	}
	for _, attr := range block.Attributes {
		if _, ok := validAttrs[attr.Name]; !ok {
			return errors.E(
				ErrUnrecognizedDefineBundleScaffoldingAttribute,
				attr.Range,
				"valid attributes are [%s] but found %q",
				strings.Join(slices.Sorted(maps.Keys(validAttrs)), ", "),
				attr.Name,
			)
		}
		if err := setAttr(validAttrs[attr.Name], attr, block.Type); err != nil {
			return err
		}
	}
	if r.Condition == nil {
		return errors.E(
			block.RawOrigins[0].DefRange(),
			"condition attribute is missing",
		)
	}
	if r.ErrorMessage == nil {
		return errors.E(
			block.RawOrigins[0].DefRange(),
			"error_message attribute is missing",
		)
	}

	*conditions = append(*conditions, &r)
	return nil
}

func parseDefineSchemaBlock(label ast.LabelBlockType, block *ast.MergedBlock) (*DefineSchema, error) {
	var name string
	if label.NumLabels == 2 {
		name = label.Labels[1]
	} else {
		name = label.Labels[0]
	}

	ret := &DefineSchema{
		Name:     name,
		DefRange: block.RawOrigins[0].DefRange(),
	}

	validAttrs := map[string]**ast.Attribute{
		"description": &ret.Description,
		"type":        &ret.Type,
	}
	if err := parseBlockAttributes(block, validAttrs, ErrUnrecognizedSchemaAttribute); err != nil {
		return nil, err
	}

	for labels, subBlock := range block.Blocks {
		switch subBlock.Type {
		case "attribute":
			attr, err := parseDefineObjectAttributeBlock(labels, subBlock)
			if err != nil {
				return nil, err
			}
			ret.ObjectAttributes = append(ret.ObjectAttributes, attr)
		default:
			return nil, errors.E(
				ErrUnrecognizedDefineSubBlock,
				subBlock.RawOrigins[0].DefRange(),
				`unexpected block type %q, expected "attribute"`,
				subBlock.Type,
			)
		}
	}
	return ret, nil
}

func parseDefineObjectAttributeBlock(label ast.LabelBlockType, block *ast.MergedBlock) (*DefineObjectAttribute, error) {
	if label.NumLabels != 1 {
		return nil, errors.E(
			block.RawOrigins[0].DefRange(),
			`attribute block must have exactly one label`,
		)
	}
	ret := &DefineObjectAttribute{
		Name:     label.Labels[0],
		DefRange: block.RawOrigins[0].DefRange(),
	}

	validAttrs := map[string]**ast.Attribute{
		"description": &ret.Description,
		"type":        &ret.Type,
		"default":     &ret.Default,
		"required":    &ret.Required,
	}
	if err := parseBlockAttributes(block, validAttrs, ErrUnrecognizedSchemaAttribute); err != nil {
		return nil, err
	}
	return ret, nil
}

func parseUsesSchemasBlock(uses *[]*UsesSchemas, block *ast.MergedBlock, label ast.LabelBlockType) error {
	if label.NumLabels != 2 {
		return errors.E(
			block.RawOrigins[0].DefRange(),
			`expected uses schema block`,
		)
	}
	if label.Labels[0] != "schemas" {
		return errors.E(
			block.RawOrigins[0].LabelRanges(),
			`unexpected label %q, expected "schemas"`,
			label.Labels[0],
		)
	}

	ret := &UsesSchemas{
		Name:     label.Labels[1],
		DefRange: block.RawOrigins[0].DefRange(),
	}

	validAttrs := map[string]**ast.Attribute{
		"source": &ret.Source,
	}
	if err := parseBlockAttributes(block, validAttrs, ErrUnrecognizedUsesAttribute); err != nil {
		return err
	}

	if ret.Source == nil {
		return errors.E(ErrMissingAttribute("uses", "source"), block.RawOrigins[0].TypeRange)
	}

	*uses = append(*uses, ret)
	return nil
}

// IsDefined checks if the metadata has any defined fields.
func (m Metadata) IsDefined() bool {
	return m.Class != nil || m.Name != nil || m.Description != nil
}

// Validate validates all parsed define blocks.
func (d *DefineBlockParser) Validate(_ *TerramateParser) error {
	return nil
}
