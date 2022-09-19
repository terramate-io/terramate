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

package globals

import (
	"github.com/mineiros-io/terramate/errors"
)

type (
	// EvalReport is the report for the evaluation globals.
	EvalReport struct {
		// Globals are the evaluated globals.
		Globals Map
		// BootstrapErr is for the case of errors happening before the evaluation.
		BootstrapErr error

		// Errors is a map of errors for each global.
		Errors map[string]EvalError // map of global name to its EvalError.
	}

	// EvalError carries the error and the expression which resulted in it.
	EvalError struct {
		Expr Expr
		Err  error
	}
)

// NewEvalReport creates a new globals evaluation report.
func NewEvalReport() EvalReport {
	return EvalReport{
		Globals: make(Map),
		Errors:  make(map[string]EvalError),
	}
}

// AsError returns an error != nil if there's any error in the report.
func (r *EvalReport) AsError() error {
	if len(r.Errors) == 0 && r.BootstrapErr == nil {
		return nil
	}

	errs := errors.L(r.BootstrapErr)
	for _, e := range r.Errors {
		errs.AppendWrap(ErrEval, e.Err)
	}
	return errs.AsError()
}
