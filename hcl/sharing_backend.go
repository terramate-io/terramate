// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package hcl

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/zclconf/go-cty/cty"
)

// SharingIsCaringExperimentName is the name of the outputs-sharing experiment.
const SharingIsCaringExperimentName = "outputs-sharing"

func (p *TerramateParser) parseSharingBackendBlock(block *ast.Block) (SharingBackend, error) {
	if !p.hasExperimentalFeature(SharingIsCaringExperimentName) {
		return SharingBackend{}, errors.E(
			ErrTerramateSchema,
			block.DefRange(),
			"unrecognized block %q (%s is an experimental feature, it must be enabled before usage with `terramate.config.experiments = [%q]`)",
			block.Type,
			SharingIsCaringExperimentName,
			SharingIsCaringExperimentName,
		)
	}
	shr := SharingBackend{}
	errs := errors.L()
	if len(block.Labels) != 1 {
		errs.Append(errors.E(
			ErrTerramateSchema,
			block.Range,
			"expected a single label but %d given",
			len(block.Labels),
		))
	} else {
		shr.Name = block.Labels[0]
	}
	isCommandDefined := false
	isFilenameDefined := false
	isTypeDefined := false
	for _, attr := range block.Attributes {
		attr := attr
		switch attr.Name {
		case "type":
			isTypeDefined = true
			val := hcl.ExprAsKeyword(attr.Expr)
			if val != TerraformSharingBackend.String() {
				errs.Append(errors.E(ErrTerramateSchema, attr.Expr.Range(), "unrecognized sharing backend type: %s", val))
				continue
			}
			shr.Type = TerraformSharingBackend
		case "filename":
			isFilenameDefined = true
			val, err := p.evalctx.Eval(attr.Expr)
			if err != nil {
				errs.Append(err)
				continue
			}
			if !val.Type().Equals(cty.String) {
				errs.Append(errors.E(ErrTerramateSchema, attr.Expr.Range(), `"filename" must be a string but %s given`, val.Type().FriendlyName()))
				continue
			}
			shr.Filename = val.AsString()
		case "command":
			isCommandDefined = true
			val, err := p.evalStringList(attr.Expr, `sharing_backend.command`)
			if err != nil {
				errs.Append(errors.E(ErrTerramateSchema, attr.Expr.Range(), err.Error()))
				continue
			}
			shr.Command = val
		default:
			errs.Append(errors.E(ErrTerramateSchema, attr.NameRange, "unrecognized attribute %s", attr.Name))
		}
	}
	if !isCommandDefined {
		errs.Append(errors.E(ErrTerramateSchema, `attribute "sharing_backend.command" is required`))
	}
	if !isFilenameDefined {
		errs.Append(errors.E(ErrTerramateSchema, `attribute "sharing_backend.filename" is required`))
	}
	if shr.Filename == "" {
		errs.Append(errors.E(ErrTerramateSchema, `empty "sharing_backend".filename`))
	}
	if !isTypeDefined {
		errs.Append(errors.E(ErrTerramateSchema, `attribute "sharing_backend.type" is required`))
	}
	if err := errs.AsError(); err != nil {
		return SharingBackend{}, err
	}
	return shr, nil
}

func (p *TerramateParser) parseInput(block *ast.Block) (Input, error) {
	if !p.hasExperimentalFeature(SharingIsCaringExperimentName) {
		return Input{}, errors.E(ErrTerramateSchema, block.DefRange(),
			"unrecognized block %q (sharing-is-caring is an experimental feature, it must be enabled before usage with `terramate.config.experiments = [%q]`)", block.Type, SharingIsCaringExperimentName)
	}
	input := Input{
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
		input.Name = block.Labels[0]
	}
	for _, attr := range block.Attributes {
		attr := attr
		switch attr.Name {
		case "backend":
			input.Backend = attr.Expr
		case "value":
			input.Value = attr.Expr
		case "from_stack_id":
			input.FromStackID = attr.Expr
		case "sensitive":
			input.Sensitive = attr.Expr
		case "mock":
			input.Mock = attr.Expr
		default:
			errs.Append(errors.E(ErrTerramateSchema, attr.NameRange, "unrecognized attribute %s", attr.Name))
		}
	}
	if input.Backend == nil {
		errs.Append(errors.E(ErrTerramateSchema, block.Range, `attribute "input.backend" is required`))
	}
	if input.FromStackID == nil {
		errs.Append(errors.E(ErrTerramateSchema, block.Range, `attribute "input.from_stack_id" is required`))
	}
	if input.Value == nil {
		errs.Append(errors.E(ErrTerramateSchema, block.Range, `attribute "input.value" is required`))
	}
	if err := errs.AsError(); err != nil {
		return Input{}, err
	}
	return input, nil
}

func (p *TerramateParser) parseOutput(block *ast.Block) (Output, error) {
	if !p.hasExperimentalFeature(SharingIsCaringExperimentName) {
		return Output{}, errors.E(ErrTerramateSchema, block.DefRange(),
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
		return Output{}, err
	}
	return output, nil
}
