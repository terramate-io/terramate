// Copyright 2022 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/zclconf/go-cty/cty"

	hhcl "github.com/hashicorp/hcl/v2"
)

// Assert represents evaluated assert block configuration.
type Assert struct {
	Assertion bool
	Warning   bool
	Message   string

	assertionRange hhcl.Range
}

func (a Assert) String() string {
	return a.assertionRange.String()
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
		res.assertionRange = cfg.Assertion.Range()
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

func evalBool(evalctx *eval.Context, expr hhcl.Expression, name string) (bool, error) {
	if expr == nil {
		return false, errors.E(ErrSchema, "%s must be defined", name)
	}
	val, err := evalctx.Eval(expr)
	if err != nil {
		return false, errors.E(err, "evaluating %s", name)
	}
	if val.Type() != cty.Bool {
		return false, errors.E(ErrSchema, "%s must be boolean, got %v", name, val.Type())
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
		return "", errors.E(ErrSchema, "%s must be string, got %v", name, val.Type())
	}
	return val.AsString(), nil
}
