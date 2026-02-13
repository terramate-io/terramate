// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package config

import (
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"

	hhcl "github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/eval"
)

// EvalBool evaluates a boolean expression.
func EvalBool(evalctx *eval.Context, expr hhcl.Expression, name string) (bool, error) {
	if expr == nil {
		return false, errors.E(ErrSchema, "%s must be defined", name)
	}
	val, err := evalctx.Eval(expr)
	if err != nil {
		return false, errors.E(err, "evaluating %s", name)
	}
	if val.Type() != cty.Bool {
		return false, errors.E(ErrSchema, "%s must be boolean, got %v", name, val.Type().FriendlyName())
	}
	return val.True(), nil
}

// EvalString evaluates a string expression.
func EvalString(evalctx *eval.Context, expr hhcl.Expression, name string) (string, error) {
	if expr == nil {
		return "", errors.E(ErrSchema, "%s must be defined", name)
	}
	val, err := evalctx.Eval(expr)
	if err != nil {
		return "", errors.E(err, "evaluating %s", name)
	}
	if val.Type() != cty.String {
		return "", errors.E(ErrSchema, "%s must be string, got %v", name, val.Type().FriendlyName())
	}
	return val.AsString(), nil
}

func evalObject(evalctx *eval.Context, obj hhcl.Expression, name string) (cty.Value, error) {
	val, err := evalctx.Eval(obj)
	if err != nil {
		return cty.Value{}, errors.E(err, "evaluating %s", name)
	}
	if !val.Type().IsObjectType() {
		return cty.Value{}, errors.E(ErrSchema, "%s must be an object, got %v", name, val.Type().FriendlyName())
	}
	return val, nil
}

func evalStringList(evalctx *eval.Context, expr hhcl.Expression, name string) ([]string, error) {
	if expr == nil {
		return nil, errors.E(ErrSchema, "%s must be defined", name)
	}

	val, err := evalctx.Eval(expr)
	if err != nil {
		return nil, errors.E(err, "evaluating %s", name)
	}

	if !val.Type().IsTupleType() && !val.Type().IsListType() {
		return nil, errors.E(ErrSchema, expr.Range(), "%s must be a list, got %s",
			name, val.Type().FriendlyName())
	}

	errs := errors.L()
	var r []string

	it := val.ElementIterator()
	index := 0
	for it.Next() {
		_, elem := it.Element()
		if elem.Type() == cty.String {
			r = append(r, elem.AsString())
		} else {
			errs.Append(errors.E(ErrSchema, expr.Range(),
				"command must be a list(string), but element %d has type %s",
				name, index, elem.Type().FriendlyName()))
		}
		index++
	}

	if err := errs.AsError(); err != nil {
		return nil, err
	}

	return r, nil
}

func evalOptionalString(evalctx *eval.Context, expr hhcl.Expression, name string) (string, error) {
	if expr == nil {
		return "", nil
	}
	val, err := evalctx.Eval(expr)
	if err != nil {
		return "", errors.E(err, "evaluating %s", name)
	}
	if val.IsNull() {
		return "", nil
	}
	if val.Type() != cty.String {
		return "", errors.E(ErrSchema, "%s must be string, got %v", name, val.Type().FriendlyName())
	}
	return val.AsString(), nil
}

func evalOptionalStringList(evalctx *eval.Context, expr hhcl.Expression, name string) ([]string, error) {
	if expr == nil {
		return nil, nil
	}

	val, err := evalctx.Eval(expr)
	if err != nil {
		return nil, errors.E(err, "evaluating %s", name)
	}
	if val.IsNull() {
		return nil, nil
	}

	if !val.Type().IsTupleType() && !val.Type().IsListType() {
		return nil, errors.E(ErrSchema, expr.Range(), "%s must be a list, got %s",
			name, val.Type().FriendlyName())
	}

	errs := errors.L()
	var r []string

	it := val.ElementIterator()
	index := 0
	for it.Next() {
		_, elem := it.Element()
		if elem.Type() == cty.String {
			r = append(r, elem.AsString())
		} else {
			errs.Append(errors.E(ErrSchema, expr.Range(),
				"command must be a list(string), but element %d has type %s",
				name, index, elem.Type().FriendlyName()))
		}
		index++
	}

	if err := errs.AsError(); err != nil {
		return nil, err
	}

	return r, nil
}

// IsCompatibleType checks if a value can be converted to the wanted type.
func IsCompatibleType(v cty.Value, wantTyp cty.Type) (bool, error) {
	if wantTyp == cty.NilType {
		return true, nil
	}

	_, err := convert.Convert(v, wantTyp)
	if err != nil {
		msg := convert.MismatchMessage(v.Type(), wantTyp)
		return false, errors.E(msg)
	}
	return true, nil
}
