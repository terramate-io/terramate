// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package hcl

import (
	"github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/zclconf/go-cty/cty"
)

// SharingIsCaringExperimentName is the name of the outputs-sharing experiment.
const SharingIsCaringExperimentName = "outputs-sharing"

// SharingBackendBlockParser is the parser for the "sharing_backend" block.
type SharingBackendBlockParser struct{}

// NewSharingBackendBlockParser returns a new parser specification for the "sharing_backend" block.
func NewSharingBackendBlockParser() *SharingBackendBlockParser {
	return &SharingBackendBlockParser{}
}

// Name returns the type of the block.
func (*SharingBackendBlockParser) Name() string {
	return "sharing_backend"
}

// Parse parses the "outputs_sharing" block.
func (*SharingBackendBlockParser) Parse(p *TerramateParser, block *ast.Block) error {
	if !p.hasExperimentalFeature(SharingIsCaringExperimentName) {
		return errors.E(
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
		return err
	}
	p.ParsedConfig.SharingBackends = append(p.ParsedConfig.SharingBackends, shr)
	return nil
}
