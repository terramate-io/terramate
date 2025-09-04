// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stdlib

import "github.com/terramate-io/terramate/errors"

// JoinListInvalidArgumentType is returned when the second argument to tm_joinlist is not a list or tuple.
const JoinListInvalidArgumentType errors.Kind = "tm_joinlist: invalid argument type"

// JoinListInvalidArgumentTypeError creates an error for invalid argument type.
func JoinListInvalidArgumentTypeError(got string) error {
	return errors.E(JoinListInvalidArgumentType,
		"tm_joinlist: expected list(list(string)) as second argument, got %s", got)
}

// JoinListInvalidElementType is returned when an element in the list is not a list or tuple of strings.
const JoinListInvalidElementType errors.Kind = "tm_joinlist: invalid element type"

// JoinListInvalidElementTypeError creates an error for invalid element type.
func JoinListInvalidElementTypeError(index int64, got string) error {
	return errors.E(JoinListInvalidElementType,
		"tm_joinlist: expected all elements to be list(string), got %s at index %d", got, index)
}

// JoinListInvalidListElementType is returned when a list element contains non-string values.
const JoinListInvalidListElementType errors.Kind = "tm_joinlist: invalid list element type"

// JoinListInvalidListElementTypeError creates an error for invalid list element type.
func JoinListInvalidListElementTypeError(index int64, got string) error {
	return errors.E(JoinListInvalidListElementType,
		"tm_joinlist: expected all elements to be list(string), got list(%s) at index %d", got, index)
}

// JoinListInvalidTupleElementType is returned when a tuple element contains non-string values.
const JoinListInvalidTupleElementType errors.Kind = "tm_joinlist: invalid tuple element type"

// JoinListInvalidTupleElementTypeError creates an error for invalid tuple element type.
func JoinListInvalidTupleElementTypeError(listIndex int64, elementIndex int, got string) error {
	return errors.E(JoinListInvalidTupleElementType,
		"tm_joinlist: expected all elements to be list(string), got tuple with %s at element %d of index %d",
		got, elementIndex, listIndex)
}
