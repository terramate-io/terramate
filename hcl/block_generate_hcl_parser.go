// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package hcl

import (
	"github.com/terramate-io/hcl/v2/hclsyntax"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/project"
)

// GenerateHCLBlockParser is the parser for the "generate_hcl" block.
type GenerateHCLBlockParser struct{}

// NewGenerateHCLBlockParser returns a new parser specification for the "generate_hcl" block.
func NewGenerateHCLBlockParser() UnmergedBlockHandler {
	return &GenerateHCLBlockParser{}
}

// Name returns the type of the block.
func (*GenerateHCLBlockParser) Name() string {
	return "generate_hcl"
}

// Parse parses the "generate_hcl" block.
func (*GenerateHCLBlockParser) Parse(p *TerramateParser, block *ast.Block) error {
	var (
		content      *hclsyntax.Block
		asserts      []AssertConfig
		stackFilters []StackFilterConfig
	)

	err := validateGenerateHCLBlock(block)
	if err != nil {
		return err
	}

	letsConfig := NewCustomRawConfig(map[string]dupeHandler{
		"lets": (*RawConfig).mergeLabeledBlock,
	})

	errs := errors.L()
	for _, subBlock := range block.Blocks {
		switch subBlock.Type {
		case "lets":
			errs.AppendWrap(ErrTerramateSchema, letsConfig.mergeBlocks(ast.Blocks{subBlock}))
		case "assert":
			assertParser := NewCustomAssertBlockParser(&asserts)
			errs.Append(assertParser.Parse(p, subBlock))

		case "stack_filter":
			stackFilterCfg, err := parseStackFilterConfig(subBlock)
			if err != nil {
				errs.Append(err)
				continue
			}
			stackFilters = append(stackFilters, stackFilterCfg)
		case "content":
			if content != nil {
				errs.Append(errors.E(subBlock.Range,
					"multiple generate_hcl.content blocks defined",
				))
				continue
			}
			content = subBlock.Block
		default:
			// already validated but sanity checks...
			panic(errors.E(errors.ErrInternal, "unexpected block type %s", subBlock.Type))
		}
	}

	if content == nil {
		errs.Append(
			errors.E(ErrTerramateSchema, `"generate_hcl" block requires a content block`, block.Range))
	}

	mergedLets := ast.MergedLabelBlocks{}
	for labelType, mergedBlock := range letsConfig.MergedLabelBlocks {
		if labelType.Type == "lets" {
			mergedLets[labelType] = mergedBlock

			errs.AppendWrap(ErrTerramateSchema, validateLets(mergedBlock))
		}
	}

	if err := errs.AsError(); err != nil {
		return err
	}

	lets, ok := mergedLets[ast.NewEmptyLabelBlockType("lets")]
	if !ok {
		lets = ast.NewMergedBlock("lets", []string{})
	}

	genblock := GenHCLBlock{
		Dir:          project.PrjAbsPath(p.rootdir, p.dir),
		Range:        block.Range,
		Label:        block.Labels[0],
		Lets:         lets,
		Asserts:      asserts,
		Content:      content.AsHCLBlock(),
		Condition:    block.Body.Attributes["condition"],
		Inherit:      block.Body.Attributes["inherit"],
		StackFilters: stackFilters,
	}
	p.ParsedConfig.Generate.HCLs = append(p.ParsedConfig.Generate.HCLs, genblock)
	return nil
}
