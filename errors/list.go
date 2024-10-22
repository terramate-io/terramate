// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package errors

import (
	"errors"
	"fmt"
	"strings"

	"github.com/terramate-io/hcl/v2"
)

// List represents a list of error instances that also
// implements Go's error interface.
//
// List implements Go's errors.Is protocol matching the
// target error with all the errors inside it, returning
// true if any of the errors is a match.
type List struct {
	errs []error
}

// L builds a List instance with all errs provided as arguments.
// Any nil errors on errs will be discarded.
//
// Any error of type hcl.Diagnostics will be flattened inside
// the error list, each hcl.Diagnostic will become an error.Error.
//
// Any error of type errors.List will be flattened inside
// the error list, each  will become an error.Error.
func L(errs ...error) *List {
	e := &List{}
	for _, err := range errs {
		e.Append(err)
	}
	return e
}

// Error returns the string representation of the error list.
// Only the first error message is returned, all other errors are elided.
// For a full representation of all errors use the List.Detailed method.
func (l *List) Error() string {
	if len(l.errs) == 0 {
		return ""
	}
	errmsg := l.errs[0].Error()
	if len(l.errs) == 1 {
		return errmsg
	}
	return fmt.Sprintf("%s (and %d elided errors)", errmsg, len(l.errs)-1)
}

// Errors returns all errors contained on the list.
// It flattens out the wrapped error lists inside *error.Error.
func (l *List) Errors() []error {
	var errs []error
	for _, err := range l.errs {
		var (
			e  *Error
			el *List
		)
		if errors.As(err, &el) {
			errs = append(errs, el.Errors()...)
		} else if errors.As(err, &e) {
			errs = append(errs, e)
		} else {
			errs = append(errs, err)
		}
	}
	return errs
}

// Detailed returns a detailed string representation of the error list.
// It will return all errors contained on the list as a single string.
// One error per line.
func (l *List) Detailed() string {
	if len(l.errs) == 0 {
		return ""
	}
	details := []string{"error list:"}
	for _, err := range l.errs {
		var errmsg string
		if e, ok := err.(interface{ Detailed() string }); ok {
			errmsg = e.Detailed()
		} else {
			errmsg = err.Error()
		}
		details = append(details, "\t-"+errmsg)
	}
	return strings.Join(details, "\n")
}

// Append appends the provided errs on the error list, ignoring nil values.
//
// Any error of type hcl.Diagnostics will have its hcl.Diagnostic elements added
// to the error list.
//
// Any error of type errors.List will be flattened inside
// the error list.
func (l *List) Append(errs ...error) {
	if len(errs) == 0 {
		return
	}

	for _, err := range errs {
		if err == nil {
			continue
		}

		switch e := err.(type) {
		case hcl.Diagnostics:
			if e != nil {
				for _, diag := range e.Errs() {
					l.Append(E(diag))
				}
			}
		case *List:
			if e != nil {
				for _, err := range e.errs {
					l.Append(err)
				}
			}
		case *Error:
			if e != nil {
				if el, ok := e.Err.(*List); ok {
					l.errs = append(l.errs, el.errs...)
				} else {
					l.errs = append(l.errs, e)
				}
			}
		case *hcl.Diagnostic:
			if e != nil {
				l.errs = append(l.errs, err)
			}
		default:
			l.errs = append(l.errs, err)
		}
	}
}

// AppendWrap is like Append() but wrap all errors with the provided kind.
func (l *List) AppendWrap(kind Kind, errs ...error) {
	for _, err := range errs {
		if err != nil {
			l.Append(E(kind, err))
		}
	}
}

// AsError returns the error list as an error instance if the errors
// list is non-empty.
// If the list is empty it will return nil.
func (l *List) AsError() error {
	if len(l.errs) == 0 {
		return nil
	}
	return l
}

func (l *List) len() int { return len(l.errs) }

// Is will call errors.Is for each of the errors on its list
// returning true on the first match it finds or false if no
// error inside the list matches the given target.
//
// If target is also an *error.List then the target list must have the same
// errors inside on the same order.
func (l *List) Is(target error) bool {
	if targetList, ok := target.(*List); ok {
		return equalLists(l, targetList)
	}
	for _, err := range l.errs {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

func equalLists(l, o *List) bool {
	if len(l.errs) != len(o.errs) {
		return false
	}
	for i, lerr := range l.errs {
		oerr := o.errs[i]
		if !errors.Is(lerr, oerr) {
			return false
		}
	}
	return true
}
