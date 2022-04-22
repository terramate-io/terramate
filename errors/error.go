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

// Errors represents a list of error instances that also
// implements Go's error interface.
// Errors implements Go's errors.Is protocol matching the
// target error with all the errors inside it, returning
// true if any of the errors is a match.
type Errors struct {
	errs []error
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

// List builds an Errors instance with all errs provided as arguments.
// Any nil errors on errs will be discarded.
func List(errs ...error) *Errors {
	e := &Errors{}
	for _, err := range errs {
		e.Append(err)
	}
	return e
}

// Error returns the string representation of the error list.
// Only the first error message is returned, all other errors are elided.
// For a full representation of all errors use the Errors.Detailed method.
func (e *Errors) Error() string {
	if len(e.errs) == 0 {
		return ""
	}
	errmsg := e.errs[0].Error()
	if len(e.errs) == 1 {
		return errmsg
	}
	return fmt.Sprintf("%s (and %d elided errors)", errmsg, len(e.errs)-1)
}

// Detailed returns a detailed string representation of the error list.
// It will return all errors contained on the list as a single string.
// One error per line.
func (e *Errors) Detailed() string {
	if len(e.errs) == 0 {
		return ""
	}
	details := []string{"error list:"}
	for _, err := range e.errs {
		details = append(details, "\t-"+err.Error())
	}
	return strings.Join(details, "\n")
}

// Append appends a new error on the error list.
// If the error is nil it will not be added on the error list.
func (e *Errors) Append(err error) {
	if err == nil {
		return
	}
	e.errs = append(e.errs, err)
}

// AsError returns an error instance if the errors list is non-empty.
// If the list is empty it will return nil.
func (e *Errors) AsError() error {
	if len(e.errs) == 0 {
		return nil
	}
	return e
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
//		errors.StackMeta
//		The stack that originated the error.
//	error
//		The underlying error that triggered this one.
//	hcl.Diagnostics
//		The underlying hcl error that triggered this one.
//		If hcl.Range is not set, the diagnostic subject range is pulled.
//		If the string Description is not set, the diagnostic detail field is
//		pulled.
//	string
//		The error description. It supports formatting using the Go's fmt verbs
//		as long as the arguments are not one of the defined types.
//
// If the error is printed, only those items that have been
// set to non-zero values will appear in the result. For the `hcl.Range` type,
// the `range.Empty()` method is used.
//
// The following fields are promoted from underlying errors when absent:
//
// - errors.Kind
// - errors.StackMeta
// - hcl.Range
//
// Minimization:
//
// In order to avoid duplicated messages, we erase the fields present in the
// underlying error if already set with same value in this error.
func E(args ...interface{}) error {
	if len(args) == 0 {
		panic("called with no args")
	}

	var (
		diags  hcl.Diagnostics
		format *string
	)

	fmtargs := []interface{}{}

	e := &Error{}
	for _, arg := range args {
		switch arg := arg.(type) {
		case Kind:
			e.Kind = arg
		case hcl.Range:
			e.FileRange = arg
		case hcl.Diagnostics:
			diags = arg
			if diags.HasErrors() {
				diag := diags[0]
				if diag.Subject != nil {
					e.FileRange = *diag.Subject
				}
			}
		case StackMeta:
			e.Stack = arg
		case error:
			e.Err = arg
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
		panic(errors.New("errors.E called with arbitraty types and no format"))
	}

	if diags.HasErrors() {
		if e.Description == "" {
			e.Description = diags[0].Detail
		} else if e.Err == nil {
			e.Err = diags
		}
	}

	if e.isEmpty() {
		panic(errors.New("empty error"))
	}
	prev, ok := e.Err.(*Error)
	if !ok {
		return e
	}
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

	return e
}

// isEmpty tells if all fields of this error are empty.
// Note that e.Err is the underlying error hence not checked.
func (e *Error) isEmpty() bool {
	return e.FileRange == hcl.Range{} && e.Kind == "" && e.Description == "" && e.Stack == nil
}

func (e *Error) error(verbose bool) string {
	var errParts []string
	for _, arg := range []interface{}{
		e.FileRange,
		e.Kind,
		e.Description,
		e.Stack,
		e.Err,
	} {
		emptyRange := hcl.Range{}
		switch v := arg.(type) {
		case hcl.Range:
			if v != emptyRange {
				if verbose {
					errParts = append(errParts,
						fmt.Sprintf("filename=%q, start line=%d, start col=%d, "+
							"start byte=%d, end line=%d, end col=%d, end byte=%d",
							v.Filename,
							v.Start.Line, v.Start.Column, v.Start.Byte,
							v.End.Line, v.End.Column, v.End.Byte),
					)
				} else {
					errParts = append(errParts, v.String())
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
				errParts = append(errParts, v.Error())
			}
		case nil:
			// ignore nil values
		default:
			panic(fmt.Errorf("unexpected errors.E type: %+v", arg))
		}
	}

	return strings.Join(errParts, separator)
}

// Error returns the error message.
func (e *Error) Error() string {
	return e.error(false)
}

// Detailed returns a detailed error message.
func (e *Error) Detailed() string {
	return e.error(true)
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
