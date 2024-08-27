// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package config

import (
	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/hcl/info"
	"github.com/zclconf/go-cty/cty"
)

type (
	// Input represents an evaluated input block
	Input struct {
		info.Range
		Name        string
		Backend     string
		FromStackID string
		value       hhcl.Expression
		Sensitive   bool
		mock        hhcl.Expression
	}

	// Inputs is a list of Input.
	Inputs []Input

	// Output represents an evaluated output block
	Output struct {
		info.Range
		Name        string
		Description string
		Backend     string
		Value       hhcl.Expression
		Sensitive   bool
	}

	// Outputs is a list of outputs.
	Outputs []Output
)

// EvalInput evaluates an input block using the provided evaluation context.
func EvalInput(evalctx *eval.Context, input hcl.Input) (Input, error) {
	evaluatedInput := Input{
		Range:     input.Range,
		Name:      input.Name, // TODO(i4k): validate name.
		Sensitive: true,
		value:     input.Value,
		mock:      input.Mock,
	}
	var err error
	errs := errors.L()
	evaluatedInput.Backend, err = evalString(evalctx, input.Backend, "input.backend")
	errs.Append(err)
	evaluatedInput.FromStackID, err = evalString(evalctx, input.FromStackID, "input.from_stack_id")
	errs.Append(err)
	errs.Append(validateID(evaluatedInput.FromStackID, "input.from_stack_id"))

	if input.Sensitive != nil {
		evaluatedInput.Sensitive, err = evalBool(evalctx, input.Sensitive, "input.sensitive")
		errs.Append(err)
	}
	if err := errs.AsError(); err != nil {
		return Input{}, err
	}
	return evaluatedInput, nil
}

// Value evaluates and returns the actual value for the input given the outputs.
func (i *Input) Value(evalctx *eval.Context) (cty.Value, error) {
	value, err := evalctx.Eval(i.value)
	if err != nil {
		return cty.NilVal, errors.E(err, `evaluating input value`)
	}
	return value, nil
}

// Mock evaluates and returns the mock value, if any.
// The returned boolean will be true only iff the mock exists in the config.
func (i *Input) Mock(evalctx *eval.Context) (cty.Value, bool, error) {
	if i.mock == nil {
		return cty.NilVal, false, nil
	}
	mock, err := evalctx.Eval(i.mock)
	if err != nil {
		return cty.NilVal, true, errors.E(err, `evaluating "input.mock"`)
	}
	return mock, true, nil
}

// EvalOutput evaluates an output block using the provided evaluation context.
func EvalOutput(evalctx *eval.Context, output hcl.Output) (Output, error) {
	evaluatedOutput := Output{
		Name:      output.Name,
		Sensitive: true,
		Value:     output.Value,
	}
	var err error
	errs := errors.L()
	if output.Description != nil {
		evaluatedOutput.Description, err = evalString(evalctx, output.Description, "output.description")
		errs.Append(err)
	}
	if output.Sensitive != nil {
		evaluatedOutput.Sensitive, err = evalBool(evalctx, output.Sensitive, "output.sensitive")
		errs.Append(err)
	}
	evaluatedOutput.Backend, err = evalString(evalctx, output.Backend, "output.backend")
	errs.Append(err)
	if err := errs.AsError(); err != nil {
		return Output{}, err
	}
	return evaluatedOutput, nil
}
