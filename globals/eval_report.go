package globals

import (
	"github.com/mineiros-io/terramate/errors"
)

type (
	EvalReport struct {
		Evaluated    G
		BootstrapErr error
		Errors       map[string]EvalError // map of global name to its EvalError.
	}

	EvalError struct {
		Expr Expr
		Err  error
	}
)

func NewEvalReport() EvalReport {
	return EvalReport{
		Evaluated: make(G),
		Errors:    make(map[string]EvalError),
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
