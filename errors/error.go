package errors

import (
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2"
)

type Error struct {
	// Kind is the kind of error.
	Kind Kind

	// FileRange holds the error source.
	FileRange hcl.Range

	// Stack which originated the error.
	Stack Stack

	// Description of the error.
	Description string

	// Err represents the underlying error.
	Err error
}

type Stack string

// Kind defines the kind of error.
type Kind uint8

const (
	Any Kind = iota
	HCLSyntax
	TerramateSchema
	TerraformSchema
	ExprEval
	ExprPartialEval
	InterpolationEval
	ForExprDisallowEval
)

const Separator = ": "

func (k Kind) String() string {
	switch k {
	case Any:
		return "unspecified error"
	case HCLSyntax:
		return "HCL syntax error"
	case TerramateSchema:
		return "Terramate schema error"
	case TerraformSchema:
		return "Terraform schema error"
	case ExprEval:
		return "expression evaluation error"
	case ExprPartialEval:
		return "expression partial evaluation error"
	case InterpolationEval:
		return "interpolation error"
	case ForExprDisallowEval:
		return "`for` expression disallow globals/terramate variables"
	}
	return fmt.Sprintf("unknown error %d", k)
}

// E builds an error value from its arguments.
// There must be at least one argument or E panics.
// The type of each argument determines its meaning. If more than one argument
// of a given type is presented, only the last one is recorded.
//
// The types are:
// 	errors.Kind
//		The kind of error (eg.: HCLSyntax, TerramateSchema, etc).
//	hcl.Range
//		The file range where the error originated.
//	string
//		Treated as an error description and assigned to the Description field if
//		not empty.
//  errors.Stack
//		The stack which the error originated.
//	error
//		The underlying error that triggered this one.
//	hcl.Diagnostics
//		The underlying hcl error that triggered this one.
//		If this error's FileRange is not set, the diagnostic subject range is
//		pulled.
//		If this error's Description is not set, the diagnostic detail field is
//		pulled.
//
// If the error is printed, only those items that have been
// set to non-zero values will appear in the result. For the `hcl.Range` type,
// the `range.Empty()` method is used.
//
// Error promotions:
//
// If Kind is not specified or Any, we set it to the Kind of
// the underlying error.
// If FileRange is not specified, we set it to the FileRange of the underlying
// error (if set).
//
// Minimization:
//
// In order to avoid duplicated messages, we erase the fields present in the
// underlying error if already set with same value in this error.
func E(args ...interface{}) error {
	if len(args) == 0 {
		panic("called with no args")
	}

	e := &Error{}
	for _, arg := range args {
		switch arg := arg.(type) {
		case Kind:
			e.Kind = arg
		case hcl.Range:
			e.FileRange = arg
		case hcl.Diagnostics:
			diags := arg
			if len(diags) > 0 {
				diag := diags[0]
				if diag.Subject != nil {
					e.FileRange = *diag.Subject
				}
				if e.Description == "" {
					e.Description = diag.Detail
				} else {
					e.Err = diags
				}
			}

		case Stack:
			e.Stack = arg
		case string:
			e.Description = arg
		case error:
			e.Err = arg
		default:
			panic(fmt.Errorf("called with unknown type %T", arg))
		}
	}

	if e.isEmpty() {
		panic(fmt.Errorf("empty error"))
	}

	prev, ok := e.Err.(*Error)
	if !ok {
		return e
	}

	if prev.Kind == e.Kind {
		prev.Kind = Any
	} else if e.Kind == Any {
		e.Kind = prev.Kind
		prev.Kind = Any
	}
	if e.FileRange.Empty() {
		e.FileRange = prev.FileRange
	}
	if prev.FileRange == e.FileRange {
		prev.FileRange = hcl.Range{}
	}
	if prev.Stack == e.Stack {
		prev.Stack = ""
	}

	if prev.Description == e.Description {
		prev.Description = ""
	}

	if prev.isEmpty() {
		e.Err = prev.Err
	}

	// If this error has Kind unset or Other, pull up the inner one.
	if e.Kind == Any {
		e.Kind = prev.Kind
		prev.Kind = Any
	}
	return e
}

// isEmpty tells if all fields of this error are empty.
// Note that e.Err is the underlying error hence not checked.
func (e *Error) isEmpty() bool {
	return e.Kind == Any && e.Description == "" && e.Stack == ""
}

// Error returns the error message.
func (e *Error) Error() string {
	if e.isEmpty() {
		if e.Err != nil {
			return e.Err.Error()
		}
		return ""
	}

	var errParts []string
	for _, arg := range []interface{}{
		e.FileRange,
		e.Kind,
		e.Description,
		e.Stack,
		e.Err,
	} {
		switch v := arg.(type) {
		case hcl.Range:
			if !v.Empty() {
				errParts = append(errParts, v.String())
			}
		case Kind:
			if v != Any {
				errParts = append(errParts, v.String())
			}
		case string:
			if v != "" {
				errParts = append(errParts, v)
			}
		case Stack:
			if v != "" {
				errParts = append(errParts, string(v))
			}
		case error:
			if v != nil {
				errParts = append(errParts, v.Error())
			}
		case nil:
			// ignore nil values
		default:
			panic(fmt.Sprintf("unexpected case: %+v", arg))
		}
	}

	return strings.Join(errParts, Separator)
}

// IsKind tells if err is of kind k.
// It returns false if err is nil or not an *errors.Error.
// It also recursively checks if any underlying error is of kind k.
func IsKind(err error, k Kind) bool {
	if err == nil {
		return false
	}

	e, ok := err.(*Error)
	if !ok {
		return false
	}

	if e.Kind != Any {
		return e.Kind == k
	}

	return IsKind(e.Err, k)
}

func Is(err, target error) bool {
	e, ok := err.(*Error)
	if !ok {
		return errors.Is(err, target)
	}
	t, ok := target.(*Error)
	if !ok {
		// target is not *Error so it's already different.
		// Let's check the underlying error.
		if e.Err != nil {
			return Is(e.Err, target)
		}
		return false
	}

	// both are *Error
	if IsKind(e, t.Kind) {
		return true
	}
	if e.Err != nil {
		return Is(e.Err, t)
	}
	return false
}
