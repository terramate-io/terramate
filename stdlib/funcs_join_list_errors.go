// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stdlib

import "fmt"

// JoinListInvalidArgumentTypeError is returned when the second argument to tm_joinlist is not a list or tuple.
type JoinListInvalidArgumentTypeError struct {
	Got string
}

func (e JoinListInvalidArgumentTypeError) Error() string {
	return fmt.Sprintf("tm_joinlist: expected list(list(string)) as second argument, got %s", e.Got)
}

// JoinListInvalidElementTypeError is returned when an element in the list is not a list or tuple of strings.
type JoinListInvalidElementTypeError struct {
	Index int64
	Got   string
}

func (e JoinListInvalidElementTypeError) Error() string {
	return fmt.Sprintf("tm_joinlist: expected all elements to be list(string), got %s at index %d", e.Got, e.Index)
}

// JoinListInvalidListElementTypeError is returned when a list element contains non-string values.
type JoinListInvalidListElementTypeError struct {
	Index int64
	Got   string
}

func (e JoinListInvalidListElementTypeError) Error() string {
	return fmt.Sprintf("tm_joinlist: expected all elements to be list(string), got list(%s) at index %d", e.Got, e.Index)
}

// JoinListInvalidTupleElementTypeError is returned when a tuple element contains non-string values.
type JoinListInvalidTupleElementTypeError struct {
	ListIndex    int64
	ElementIndex int
	Got          string
}

func (e JoinListInvalidTupleElementTypeError) Error() string {
	return fmt.Sprintf("tm_joinlist: expected all elements to be list(string), got tuple with %s at element %d of index %d",
		e.Got, e.ElementIndex, e.ListIndex)
}
