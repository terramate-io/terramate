// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package config

import (
	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/zclconf/go-cty/cty"
)

func evalBool(evalctx *eval.Context, expr hhcl.Expression, name string) (bool, error) {
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

func evalString(evalctx *eval.Context, expr hhcl.Expression, name string) (string, error) {
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
