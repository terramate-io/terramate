// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package hcl

import (
	"path/filepath"

	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/stdlib"
	"github.com/zclconf/go-cty/cty"
)

// StackBlockSpec is the specification for the stack block.
type StackBlockSpec struct {
}

// NewStackBlockParser returns a new parser specification for the "stack" block.
func NewStackBlockParser() *StackBlockSpec {
	return &StackBlockSpec{}
}

// Name returns the name of the block.
func (*StackBlockSpec) Name() string {
	return "stack"
}

// NewStackBlockConstructor returns a new stack block handler.
func NewStackBlockConstructor() UnmergedBlockHandler {
	return NewStackBlockParser()
}

// Parse parses the "stack" block.
func (*StackBlockSpec) Parse(p *TerramateParser, block *ast.Block) error {
	if p.ParsedConfig.Stack != nil {
		return errors.E(ErrTerramateSchema, block.DefRange(),
			"duplicated stack blocks across configs")
	}

	// We store Pro attributes for the stack in the p.ParsedConfig.External.Stacks map.

	errs := errors.L()
	for _, block := range block.Body.Blocks {
		errs.Append(
			errors.E(ErrTerramateSchema, block.TypeRange, "unrecognized block %q", block.Type),
		)
	}

	stackdir := filepath.Dir(block.Range.HostPath())
	evalctx := eval.NewContext(stdlib.Functions(stackdir, p.ParsedConfig.Experiments()))

	stack := &Stack{}
	var bundleStack *BundleStack

	attrs := ast.AsHCLAttributes(block.Body.Attributes)
	for _, attr := range ast.SortRawAttributes(attrs) {
		attrVal, err := evalctx.Eval(attr.Expr)
		if err != nil {
			errs.Append(
				errors.E(ErrTerramateSchema, err, "failed to evaluate %q attribute", attr.Name),
			)
			continue
		}

		switch attr.Name {
		// pro-only attributes
		case "from_bundle_file":
			if attrVal.Type() != cty.String {
				errs.Append(hclAttrErr(attr,
					"field stack.from_bundle_file must be a string but is %q",
					attrVal.Type().FriendlyName()),
				)
				continue
			}
			if bundleStack == nil {
				bundleStack = &BundleStack{}
			}

			bundleStack.FromBundleFile = attrVal.AsString()

		case "from_bundle_label":
			if attrVal.Type() != cty.String {
				errs.Append(hclAttrErr(attr,
					"field stack.from_bundle_label must be a string but is %q",
					attrVal.Type().FriendlyName()),
				)
				continue
			}
			if bundleStack == nil {
				bundleStack = &BundleStack{}
			}
			bundleStack.FromBundleLabel = attrVal.AsString()
		case "id":
			if attrVal.Type() != cty.String {
				errs.Append(hclAttrErr(attr,
					"field stack.id must be a string but is %q",
					attrVal.Type().FriendlyName()),
				)
				continue
			}
			stack.ID = attrVal.AsString()
		case "name":
			if attrVal.Type() != cty.String {
				errs.Append(hclAttrErr(attr,
					"field stack.name must be a string but given %q",
					attrVal.Type().FriendlyName()),
				)
				continue
			}
			stack.Name = attrVal.AsString()

		case "description":
			if attrVal.Type() != cty.String {
				errs.Append(hclAttrErr(attr,
					"field stack.\"description\" must be a \"string\" but given %q",
					attrVal.Type().FriendlyName(),
				))

				continue
			}
			stack.Description = attrVal.AsString()

			// The `tags`, `after`, `before`, `wants`, `wanted_by` and `watch`
			// have all the same parsing rules.
			// By the spec, they must be a `set(string)`.

			// In order to speed up the tests, only the `after` attribute is
			// extensively tested for all error cases.
			// **So have this in mind if the specification of any of the attributes
			// below change in the future**.

		case "tags":
			errs.Append(assignSet(attr, &stack.Tags, attrVal))

		case "after":
			errs.Append(assignSet(attr, &stack.After, attrVal))

		case "before":
			errs.Append(assignSet(attr, &stack.Before, attrVal))

		case "wants":
			errs.Append(assignSet(attr, &stack.Wants, attrVal))

		case "wanted_by":
			errs.Append(assignSet(attr, &stack.WantedBy, attrVal))

		case "watch":
			errs.Append(assignSet(attr, &stack.Watch, attrVal))

		default:
			errs.Append(
				errors.E(ErrTerramateSchema, attr.NameRange, "unrecognized attribute stack.%q", attr.Name),
			)
		}
	}

	if err := errs.AsError(); err != nil {
		return err
	}
	for _, block := range block.Body.Blocks {
		errs.Append(
			errors.E(block.TypeRange, "unrecognized block %q", block.Type),
		)
	}

	p.ParsedConfig.Stack = stack

	if bundleStack != nil {
		if p.ParsedConfig.BundleStacks == nil {
			p.ParsedConfig.BundleStacks = make(map[string]BundleStack)
		}
		p.ParsedConfig.BundleStacks[stackdir] = *bundleStack
	}
	return nil
}
