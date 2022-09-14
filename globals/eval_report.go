package globals

import (
	"github.com/mineiros-io/terramate/errors"
)

type (
	// EvalReport is the report for the evaluation globals.
	EvalReport struct {
		// Globals are the evaluated globals.
		Globals G
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
		Globals: make(G),
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
		errs.AppendWrap(ErrGlobalEval, e.Err)
	}
	return errs.AsError()
}
