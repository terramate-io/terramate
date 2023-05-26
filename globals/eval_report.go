// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package globals

import (
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/project"

	"github.com/terramate-io/terramate/hcl/eval"
)

type (
	// EvalReport is the report for the evaluation globals.
	EvalReport struct {
		// Globals are the evaluated globals.
		Globals *eval.Object

		// BootstrapErr is for the case of errors happening before the evaluation.
		BootstrapErr error

		// Errors is a map of errors for each global.
		Errors map[GlobalPathKey]EvalError // map of GlobalPath to its EvalError.
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
		Globals: eval.NewObject(eval.Info{
			DefinedAt: project.NewPath("/"),
		}),
		Errors: make(map[GlobalPathKey]EvalError),
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
