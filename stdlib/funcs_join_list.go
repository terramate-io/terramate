// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stdlib

import (
	"strings"

	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

// JoinListFunc implements the `tm_joinlist()` function.
// tm_joinlist(separator: string, list_of_lists: list(list(string))) -> list(string)
// It joins each sublist in the list_of_lists using the separator.
func JoinListFunc() function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "separator",
				Type: cty.String,
			},
			{
				Name: "list_of_lists",
				Type: cty.DynamicPseudoType,
			},
		},
		Type: function.StaticReturnType(cty.List(cty.String)),
		Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
			return joinList(args[0], args[1])
		},
	})
}

func joinList(separatorVal, listOfListsVal cty.Value) (cty.Value, error) {
	separator := separatorVal.AsString()

	// Validate that list_of_lists is actually a list or tuple (HCL represents [["a"]] as tuple)
	if !listOfListsVal.Type().IsListType() && !listOfListsVal.Type().IsTupleType() {
		return cty.NilVal, JoinListInvalidArgumentTypeError(listOfListsVal.Type().FriendlyName())
	}

	// Handle empty input list
	if listOfListsVal.LengthInt() == 0 {
		return cty.ListValEmpty(cty.String), nil
	}

	var results []cty.Value
	listOfListsIt := listOfListsVal.ElementIterator()

	for listOfListsIt.Next() {
		idx, subListVal := listOfListsIt.Element()

		// Type validation - ensure each element is list(string) or tuple(string...)
		if !subListVal.Type().IsListType() && !subListVal.Type().IsTupleType() {
			idxInt, _ := idx.AsBigFloat().Int64()
			return cty.NilVal, JoinListInvalidElementTypeError(idxInt, subListVal.Type().FriendlyName())
		}

		// For lists, check element type
		if subListVal.Type().IsListType() {
			if !subListVal.Type().ElementType().Equals(cty.String) {
				idxInt, _ := idx.AsBigFloat().Int64()
				return cty.NilVal, JoinListInvalidListElementTypeError(idxInt, subListVal.Type().ElementType().FriendlyName())
			}
		}

		// For tuples, check all element types are strings
		if subListVal.Type().IsTupleType() {
			for i, elemType := range subListVal.Type().TupleElementTypes() {
				if !elemType.Equals(cty.String) {
					idxInt, _ := idx.AsBigFloat().Int64()
					return cty.NilVal, JoinListInvalidTupleElementTypeError(idxInt, i, elemType.FriendlyName())
				}
			}
		}

		// Handle empty sublist
		if subListVal.LengthInt() == 0 {
			results = append(results, cty.StringVal(""))
			continue
		}

		// Join elements of the sublist
		var subElements []string
		subListIt := subListVal.ElementIterator()
		for subListIt.Next() {
			_, elementVal := subListIt.Element()
			subElements = append(subElements, elementVal.AsString())
		}

		joinedString := strings.Join(subElements, separator)
		results = append(results, cty.StringVal(joinedString))
	}

	return cty.ListVal(results), nil
}
