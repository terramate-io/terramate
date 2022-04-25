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

package errors

import (
	"errors"
	"fmt"
	"strings"
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

// Errors returns all errors contained on the list that are of the type Error
// or that have an error of type Error wrapped inside them.
// Any other errors will be ignored.
func (l *List) Errors() []*Error {
	var errs []*Error
	for _, err := range l.errs {
		var e *Error
		if errors.As(err, &e) {
			errs = append(errs, e)
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
		details = append(details, "\t-"+err.Error())
	}
	return strings.Join(details, "\n")
}

// Append appends a new error on the error list.
// If the error is nil it will not be added on the error list.
func (l *List) Append(err error) {
	if err == nil {
		return
	}
	l.errs = append(l.errs, err)
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

// Is will call errors.Is for each of the errors on its list
// returning true on the first match it finds or false if no
// error inside the list matches the given target.
//
// If the target error is nil and the error list is empty returns true.
func (l *List) Is(target error) bool {
	for _, err := range l.errs {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}
