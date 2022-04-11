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
	stdfmt "fmt"
	"strings"

	"github.com/hashicorp/hcl/v2"
)

// Error is the default Terramate error type.
// It may contains some fields unset, but not all.
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

	// Err represents the underlying error.
	Err error
}

type (
	// Kind defines the kind of error.
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
		case StackMeta:
			e.Stack = arg
		case string:
			e.Description = arg
		case error:
			e.Err = arg
		default:
			panic(stdfmt.Errorf("called with unknown type %T", arg))
		}
	}
	if e.isEmpty() {
		panic(errors.New("empty error"))
	}
	prev, ok := e.Err.(*Error)
	if !ok {
		return e
	}
	if prev.Kind == e.Kind {
		prev.Kind = ""
	}
	if e.Kind == "" {
		e.Kind = prev.Kind
		prev.Kind = ""
	}
	if e.FileRange.Empty() {
		e.FileRange = prev.FileRange
	}
	if prev.FileRange == e.FileRange {
		prev.FileRange = hcl.Range{}
	}
	if prev.Stack == e.Stack {
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
	return e.FileRange.Empty() && e.Kind == "" && e.Description == "" && e.Stack == nil
}

func (e *Error) error(verbose bool) string {
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
				if verbose {
					errParts = append(errParts, fmt(
						"filename=%q,start line=%d,start col=%d,start byte=%d,end line=%d,end col=%d,end byte=%d",
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
						fmt("at stack (name=%q,path=%q,desc=%q)",
							v.Name(), v.Path(), v.Desc(),
						))
				} else {
					errParts = append(errParts, fmt("at stack %q", v.Path()))
				}
			}
		case error:
			if v != nil {
				errParts = append(errParts, v.Error())
			}
		case nil:
			// ignore nil values
		default:
			panic(fmt("unexpected case: %+v", arg))
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
	if e.Kind != "" && e.Kind == k {
		return true
	}
	return IsKind(e.Err, k)
}

// Is tells if err (or any of its underlying errors) matches target.
// It works with any error but if comparing *Error type it uses the Kind field,
// otherwise it fallback to standard errors.Is().
func Is(err, target error) bool {
	if (err == nil) != (target == nil) {
		return false
	}
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

	if t.Kind != "" && !IsKind(e, t.Kind) {
		return false
	}

	if t.Description != "" && !hasDesc(e, t.Description) {
		return false
	}

	if t.Stack != nil && !hasSameStack(e, t.Stack) {
		return false
	}

	if !t.FileRange.Empty() && e.FileRange != t.FileRange {
		return false
	}

	if t.Err != nil {
		return Is(e.Err, t.Err)
	}

	return true
}

func hasDesc(err error, desc string) bool {
	e, ok := err.(*Error)
	if !ok {
		return false
	}

	if e.Description == desc {
		return true
	}
	if e.Err != nil {
		return hasDesc(e.Err, desc)
	}
	return false
}

// hasSameStack recursively check if err or any of its underlying errors has the
// provided stack set.
func hasSameStack(err error, stack StackMeta) bool {
	e, ok := err.(*Error)
	if !ok {
		return false
	}
	if e.Stack.Name() == stack.Name() &&
		e.Stack.Desc() == stack.Desc() &&
		e.Stack.Path() == stack.Path() {
		return true
	}
	if e.Err != nil {
		return hasSameStack(e.Err, stack)
	}
	return false
}

func fmt(format string, args ...interface{}) string {
	return stdfmt.Sprintf(format, args...)
}
