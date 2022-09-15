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

// Package errors implements the Terramate standard error type.
// It's heavily influenced by Rob Pike `errors` package in the Upspin project:
// 	https://commandcenter.blogspot.com/2017/12/error-handling-in-upspin.html
package errors

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
)

// Error is the default Terramate error type.
// At least one of the error fields must be set.
// See E() for its usage.
type Error struct {
	// Kind is the kind of error.
	Kind Kind

	// Description of the error.
	Description string

	// FileRange holds the error source.
	FileRange hcl.Range

	// Stack which originated the error.
	Stack StackMeta

	// Err holds the underlying error.
	Err error
}

type (
	// Kind defines the kind of an error.
	Kind string
	// StackMeta has the metadata of the stack which originated the error.
	// Same interface as stack.Metadata.
	StackMeta interface {
		Name() string
		Desc() string
		Path() string
	}
)

const separator = ": "

// E builds an error value from its arguments.
// There must be at least one argument or E panics.
// The type of each argument determines its meaning.
// Multiple underlying errors can be provided and in such case E() builds a *List
// of errors as its underlying error.
// If multiple arguments of same type is presented (and it's not an
// underlying error type), only the last one is recorded.
//
// The supported types are:
//
// 	errors.Kind
//		The kind of error (eg.: HCLSyntax, TerramateSchema, etc).
//	hcl.Range
//		The file range where the error originated.
//	errors.StackMeta
//		The stack that originated the error.
//	hcl.Diagnostics
//		The underlying hcl error that triggered this one.
//		Only the first hcl.Diagnostic will be used.
//		If hcl.Range is not set, the diagnostic subject range is pulled.
//		If the string Description is not set, the diagnostic detail field is
//		pulled.
//	hcl.Diagnostic
//		Same behavior as hcl.Diagnostics but for a single diagnostic.
//	string
//		The error description. It supports formatting using the Go's fmt verbs
//		as long as the arguments are not one of the defined types.
//
//  The underlying error types are:
//
//  *List
//		The underlying error list wrapped by this one.
// 		This error wraps all of its individual errors so they carry all the
//		context to print them individually.
//	hcl.Diagnostics
//		The underlying list of hcl errors wrapped by this one.
//		This type is converted to a *List containing only the hcl.DiagError values.
//	hcl.Diagnostic
//		The underlying hcl error wrapped by this one.
//		It's ignored if its type is not hcl.DiagError.
//		If hcl.Range is not already set, the diagnostic subject range is pulled.
//		If the string Description is not set, the diagnostic detail field is
//		pulled.
//	error
//		The underlying error that triggered this one.
//
// If the error is printed, only those items that have been
// set to non-zero values will appear in the result. For the `hcl.Range` type,
// the `range.Empty()` method is used.
//
// When the underlying error is a single error, then the fields below are
// promoted from the underlying error when absent:
//
// - errors.Kind
// - errors.StackMeta
// - hcl.Range
//
// Minimization:
//
// In order to avoid duplicated messages, if the underlying error is an *Error,
// we erase the fields present in it if already set with same value in this
// error.
func E(args ...interface{}) *Error {
	if len(args) == 0 {
		panic("called with no args")
	}

	var format *string

	fmtargs := []interface{}{}

	e := &Error{}
	defer func() {
		if e.isEmpty() {
			panic(errors.New("empty error"))
		}
	}()

	errs := L()
	for _, arg := range args {
		switch arg := arg.(type) {
		case Kind:
			e.Kind = arg
		case hcl.Range:
			e.FileRange = arg
		case StackMeta:
			e.Stack = arg
		case hcl.Diagnostics:
			errs.Append(arg)
		case hcl.Diagnostic:
			errs.Append(&arg)
		case *hcl.Diagnostic:
			errs.Append(arg)
		case *List:
			errs.Append(arg)
		case error:
			errs.Append(arg)
		case string:
			val := arg
			if format == nil {
				format = &val
			} else {
				fmtargs = append(fmtargs, val)
			}
		default:
			fmtargs = append(fmtargs, arg)
		}
	}

	if format != nil {
		e.Description = fmt.Sprintf(*format, fmtargs...)
	} else if len(fmtargs) > 0 {
		panic(fmt.Errorf("errors.E called with arbitrary types %#v and no format", fmtargs))
	}

	if errs.len() == 0 {
		return e
	}

	if errs.len() > 1 {
		// if the underlying error is a *List we wrap all of its elements so they
		// carry all the context needed to print them individually.
		// Eg.:
		//   errs := errors.L()
		//   obj, err := something.Do()
		//   if err != nil {
		// 	 	errs.Append(errors.E(ErrSomethingBadHappened, err))
		//   }
		//
		//   if `err` is an *errors.List, the code above means that all of their
		//   error items have the kind `ErrSomethingBadHappened`.
		//

		// code below captures all arguments but the underlying errors
		// so we can wrap the elements of the list with same semantics intended
		// by the caller of E.
		wrappingArgs := []interface{}{}
		for _, arg := range args {
			switch arg.(type) {
			case error, *Error, *List, hcl.Diagnostic, *hcl.Diagnostic,
				hcl.Diagnostics:
				// do nothing
			default:
				wrappingArgs = append(wrappingArgs, arg)
			}
		}

		for i, el := range errs.errs {
			args := make([]interface{}, len(wrappingArgs))
			copy(args, wrappingArgs)
			args = append(args, el)
			errs.errs[i] = E(args...)
		}

		e.Err = errs
		return e
	}

	// errs.len == 1
	e.Err = errs.errs[0]

	switch prev := e.Err.(type) {
	case *hcl.Diagnostic:
		if prev.Subject != nil && e.FileRange.Empty() {
			e.FileRange = *prev.Subject
		}

		if e.Description == "" {
			e.Description = prev.Detail
		}

		e.Err = nil
	case *Error:
		if e.Kind == "" {
			e.Kind = prev.Kind
		}
		if prev.Kind == e.Kind {
			prev.Kind = ""
		}

		emptyRange := hcl.Range{}
		if e.FileRange == emptyRange {
			e.FileRange = prev.FileRange
		}
		if prev.FileRange == e.FileRange {
			prev.FileRange = hcl.Range{}
		}
		if equalStack(e.Stack, prev.Stack) {
			prev.Stack = nil
		}
		if prev.Description == e.Description {
			prev.Description = ""
		}
		if prev.isEmpty() {
			e.Err = prev.Err
		}
	}

	return e
}

// isEmpty tells if all fields of this error are empty.
// Note that e.Err is the underlying error hence not checked.
func (e *Error) isEmpty() bool {
	return e.FileRange == hcl.Range{} && e.Kind == "" && e.Description == "" && e.Stack == nil
}

func (e *Error) error(fields []interface{}, verbose bool) string {
	var errParts []string
	for _, arg := range fields {
		emptyRange := hcl.Range{}
		switch v := arg.(type) {
		case *List:
			// if the underlying error is a *List, all the elements are wrapped
			// with the parent error fields.
			return v.Error()
		case hcl.Range:
			if v != emptyRange {
				if verbose {
					errParts = append(errParts,
						fmt.Sprintf("filename=%q, start line=%d, start col=%d, "+
							"start byte=%d, end line=%d, end col=%d, end byte=%d",
							filename(v.Filename),
							v.Start.Line, v.Start.Column, v.Start.Byte,
							v.End.Line, v.End.Column, v.End.Byte),
					)
				} else {
					copiedRange := v
					copiedRange.Filename = filename(copiedRange.Filename)
					errParts = append(errParts, copiedRange.String())
				}
			}
		case Kind:
			if v != "" {
				errParts = append(errParts, string(v))
			}
		case string:
			if v != "" {
				errParts = append(errParts, v)
			}
		case StackMeta:
			if v != nil {
				if verbose {
					errParts = append(errParts,
						fmt.Sprintf("at stack (name=%q, path=%q, desc=%q)",
							v.Name(), v.Path(), v.Desc(),
						))
				} else {
					errParts = append(errParts, fmt.Sprintf("at stack %q", v.Path()))
				}
			}
		case error:
			if v != nil {
				errmsg := ""
				e, ok := v.(*Error)
				if ok {
					errmsg = e.error(e.defaultErrorFields(), verbose)
				} else {
					errmsg = v.Error()
				}
				errParts = append(errParts, errmsg)
			}
		case nil:
			// ignore nil values
		default:
			panic(fmt.Errorf("unexpected errors.E type: %+v", arg))
		}
	}

	return strings.Join(errParts, separator)
}

func (e *Error) defaultErrorFields() []interface{} {
	return []interface{}{
		e.FileRange,
		e.Kind,
		e.Description,
		e.Stack,
		e.Err,
	}
}

// Error returns the error message.
func (e *Error) Error() string {
	return e.error(e.defaultErrorFields(), false)
}

// Detailed returns a detailed error message.
func (e *Error) Detailed() string {
	return e.error(e.defaultErrorFields(), true)
}

// AsList returns the error as a list.
// If it's underlying error is a *List, then it just returns it because
// they're already explicitly wrapped.
func (e *Error) AsList() *List {
	var el *List
	if errors.As(e, &el) {
		return el
	}

	return L(e)
}

// Message returns the error message without some metadata.
// This method is suitable for editor extensions that needs to handle the
// metadata themselves.
func (e *Error) Message() string {
	return e.error([]interface{}{
		e.Kind,
		e.Description,
		e.Stack,
		e.Err,
	}, false)
}

// Is tells if err matches the target error.
// The target error must be of errors.Error type and it will try to match the following fields:
// - Kind
// - Description
// - Stack
// - FileRange
// Any fields absent (empty) on the target error are ignored even if they exist on err (partial match).
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}

	if t.Kind != "" && e.Kind != t.Kind {
		return false
	}

	if t.Description != "" && e.Description != t.Description {
		return false
	}

	if t.Stack != nil && !equalStack(e.Stack, t.Stack) {
		return false
	}

	if !t.FileRange.Empty() && e.FileRange != t.FileRange {
		return false
	}

	return true
}

// Unwrap returns the wrapped error, if there is any.
// Returns nil if there is no wrapped error.
func (e *Error) Unwrap() error {
	return e.Err
}

// IsKind tells if err is of kind k.
// It is a small wrapper around calling errors.Is(err, errors.Kind(k)).
func IsKind(err error, k Kind) bool {
	return Is(err, E(k))
}

// Is is just an alias to Go stdlib errors.Is
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// As is just an alias to Go stdlib errors.As
func As(err error, target interface{}) bool {
	return errors.As(err, target)
}

func equalStack(s1, s2 StackMeta) bool {
	if (s1 == nil) != (s2 == nil) {
		return false
	}

	if s1 == nil { // s2 is also nil
		return true
	}

	return s1.Name() == s2.Name() &&
		s1.Desc() == s2.Desc() &&
		s1.Path() == s2.Path()
}

func filename(fname string) string {
	if ext := filepath.Ext(fname); ext == ".tm" || ext == ".tm.hcl" {
		return fname
	}
	return "<generated-code>"
}
