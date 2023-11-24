// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package config

import (
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/eval"

	hhcl "github.com/hashicorp/hcl/v2"
)

// Assert represents evaluated assert block configuration.
type Assert struct {
	Assertion bool
	Warning   bool
	Message   string
	Range     hhcl.Range
}

// EvalAssert evaluates a given assert configuration and returns its
// evaluated form.
func EvalAssert(evalctx *eval.Context, cfg hcl.AssertConfig) (Assert, error) {
	res := Assert{}
	errs := errors.L()

	assertion, err := evalBool(evalctx, cfg.Assertion, "assert.assertion")
	if err != nil {
		errs.Append(err)
	} else {
		res.Assertion = assertion
		res.Range = cfg.Assertion.Range()
	}

	message, err := evalString(evalctx, cfg.Message, "assert.message")
	if err != nil {
		errs.Append(err)
	} else {
		res.Message = message
	}

	if cfg.Warning != nil {
		warning, err := evalBool(evalctx, cfg.Warning, "assert.warning")
		if err != nil {
			errs.Append(err)
		} else {
			res.Warning = warning
		}
	}

	if err := errs.AsError(); err != nil {
		return Assert{}, err
	}

	return res, nil
}
