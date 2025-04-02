// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package hcl

import (
	"github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/project"
)

// GenerateFileBlockParser is the parser for the "generate_file" block.
type GenerateFileBlockParser struct{}

// NewGenerateFileBlockParser returns a new parser specification for the "generate_file" block.
func NewGenerateFileBlockParser() *GenerateFileBlockParser {
	return &GenerateFileBlockParser{}
}

// Name returns the type of the block.
func (*GenerateFileBlockParser) Name() string {
	return "generate_file"
}

// Parse parses the "generate_file" block.
func (*GenerateFileBlockParser) Parse(p *TerramateParser, block *ast.Block) error {
	err := validateGenerateFileBlock(block)
	if err != nil {
		return err
	}

	var asserts []AssertConfig
	var stackFilters []StackFilterConfig

	letsConfig := NewCustomRawConfig(map[string]dupeHandler{
		"lets": (*RawConfig).mergeLabeledBlock,
	})

	errs := errors.L()

	context := "stack"
	if contextAttr, ok := block.Body.Attributes["context"]; ok {
		context = hcl.ExprAsKeyword(contextAttr.Expr)
		if context != "stack" && context != "root" {
			errs.Append(errors.E(contextAttr.Expr.Range(),
				"generate_file.context supported values are \"stack\" and \"root\""+
					" but given %q", context))
		}
	}

	for _, subBlock := range block.Blocks {
		switch subBlock.Type {
		case "lets":
			errs.AppendWrap(ErrTerramateSchema, letsConfig.mergeBlocks(ast.Blocks{subBlock}))
		case "assert":
			assertParser := NewAssertBlockParser(&asserts)
			errs.Append(assertParser.Parse(nil, subBlock))

		case "stack_filter":
			if context != "stack" {
				errs.Append(errors.E(ErrTerramateSchema, subBlock.Range,
					"stack_filter is only supported with context = \"stack\""))
				continue
			}
			stackFilterCfg, err := parseStackFilterConfig(subBlock)
			if err != nil {
				errs.Append(err)
				continue
			}
			stackFilters = append(stackFilters, stackFilterCfg)
		default:
			// already validated but sanity checks...
			panic(errors.E(errors.ErrInternal, "unexpected block type %s", subBlock.Type))
		}
	}

	mergedLets := ast.MergedLabelBlocks{}
	for labelType, mergedBlock := range letsConfig.MergedLabelBlocks {
		if labelType.Type == "lets" {
			mergedLets[labelType] = mergedBlock

			errs.AppendWrap(ErrTerramateSchema, validateLets(mergedBlock))
		}
	}

	inherit := block.Body.Attributes["inherit"]
	if inherit != nil && context == "root" {
		errs.Append(errors.E(ErrTerramateSchema,
			inherit.Range(),
			`inherit attribute cannot be used with context=root`,
		))
	}

	if err := errs.AsError(); err != nil {
		return err
	}

	lets, ok := mergedLets[ast.NewEmptyLabelBlockType("lets")]
	if !ok {
		lets = ast.NewMergedBlock("lets", []string{})
	}

	p.ParsedConfig.Generate.Files = append(p.ParsedConfig.Generate.Files, GenFileBlock{
		Dir:          project.PrjAbsPath(p.rootdir, p.dir),
		Range:        block.Range,
		Label:        block.Labels[0],
		Lets:         lets,
		Asserts:      asserts,
		StackFilters: stackFilters,
		Content:      block.Body.Attributes["content"],
		Condition:    block.Body.Attributes["condition"],
		Inherit:      inherit,
		Context:      context,
	})
	return nil
}
