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
)

// Assert represents evaluated assert block configuration.
type Assert struct {
	Assertion bool
	Warning   bool
	Message   string
}

// EvalAssert evaluates a given assert configuration and returns its
// evaluated form.
func EvalAssert(evalctx *eval.Context, cfg hcl.AssertConfig) (Assert, error) {
	res := Assert{}
	errs := errors.L()

	if cfg.Assertion != nil {
		assertionVal, err := evalctx.Eval(cfg.Assertion)
		if err != nil {
			errs.Append(errors.E(err, "evaluating assert.assertion"))
		} else {
			if assertionVal.Type() == cty.Bool {
				res.Assertion = assertionVal.True()
			} else {
				errs.Append(errors.E(ErrSchema, "assert.assertion must be boolean, got %v", assertionVal.Type()))
			}
		}
	} else {
		errs.Append(errors.E(ErrSchema, "assert.assertion must be defined"))
	}

	if cfg.Message != nil {
		messageVal, err := evalctx.Eval(cfg.Message)
		if err != nil {
			errs.Append(errors.E(err, "evaluating assert.message"))
		} else {
			if messageVal.Type() == cty.String {
				res.Message = messageVal.AsString()
			} else {
				errs.Append(errors.E(ErrSchema, "assert.message must be string, got %v", messageVal.Type()))
			}
		}
	} else {
		errs.Append(errors.E(ErrSchema, "assert.message must be defined"))
	}

	if cfg.Warning != nil {
		warningVal, err := evalctx.Eval(cfg.Warning)
		if err != nil {
			errs.Append(errors.E(err, "evaluating assert.warning"))
		} else {
			if warningVal.Type() == cty.Bool {
				res.Warning = warningVal.True()
			} else {
				return Assert{}, errors.E(ErrSchema, "assert.warning must be boolean", warningVal.Type())
			}
		}
	}

	if err := errs.AsError(); err != nil {
		return Assert{}, err
	}

	return res, nil
}
