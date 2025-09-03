// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stdlib

import (
	"regexp"
	"strings"

	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

var slugRegexp = regexp.MustCompile("[^a-z0-9_]")

// SlugFunc returns the `tm_slug` function spec.
// It accepts either a string or list(string) and returns the appropriately typed slugified result.
func SlugFunc() function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name:         "input",
				Type:         cty.DynamicPseudoType, // Accept any type for polymorphic behavior
				AllowNull:    true,                  // Allow null values to be passed through
				AllowUnknown: true,                  // Allow unknown values to be passed through
			},
		},
		Type: func(args []cty.Value) (cty.Type, error) {
			argType := args[0].Type()
			switch {
			case argType.Equals(cty.String):
				return cty.String, nil
			case argType.IsListType() && argType.ElementType().Equals(cty.String):
				return argType, nil
			case argType.IsTupleType():
				// Accept all tuples and validate in implementation for better error messages
				return cty.List(cty.String), nil
			default:
				return cty.NilType, errWrongRootType("tm_slug", argType)
			}
		},
		Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
			return tmSlug(args[0])
		},
	})
}

// tmSlug implements the polymorphic tm_slug function logic
func tmSlug(arg cty.Value) (cty.Value, error) {
	argType := arg.Type()

	if arg.IsNull() {
		// For tuples, return null List(String) to match Type function declaration
		if argType.IsTupleType() {
			return cty.NullVal(cty.List(cty.String)), nil
		}
		return cty.NullVal(argType), nil
	}

	if !arg.IsWhollyKnown() {
		// Propagate unknown values with correct output type
		switch {
		case argType.Equals(cty.String):
			return cty.UnknownVal(cty.String), nil
		case argType.IsListType() && argType.ElementType().Equals(cty.String):
			return cty.UnknownVal(argType), nil
		case argType.IsTupleType():
			return cty.UnknownVal(cty.List(cty.String)), nil
		default:
			return cty.DynamicVal, errUnknownValue("tm_slug")
		}
	}

	switch {
	case argType.Equals(cty.String):
		return cty.StringVal(slugify(arg.AsString())), nil

	case argType.IsListType() && argType.ElementType().Equals(cty.String):
		return slugifyList(arg)

	case argType.IsTupleType():
		return slugifyTuple(arg)

	default:
		return cty.NilVal, errWrongRootType("tm_slug", argType)
	}
}

// slugify converts a string to a slug using the existing regex pattern
func slugify(input string) string {
	return slugRegexp.ReplaceAllString(strings.ToLower(input), "-")
}

// slugifyList processes a list of strings and returns a list of slugified strings
func slugifyList(listVal cty.Value) (cty.Value, error) {
	if listVal.LengthInt() == 0 {
		return cty.ListValEmpty(cty.String), nil
	}

	it := listVal.ElementIterator()
	out := make([]cty.Value, 0, listVal.LengthInt())
	index := 0

	for it.Next() {
		_, v := it.Element()
		if !v.Type().Equals(cty.String) {
			return cty.NilVal, errListElemNotString("tm_slug", index, v.Type())
		}
		if v.IsNull() {
			out = append(out, cty.NullVal(cty.String))
		} else {
			out = append(out, cty.StringVal(slugify(v.AsString())))
		}
		index++
	}

	return cty.ListVal(out), nil
}

// slugifyTuple processes a tuple of strings and returns a list of slugified strings
func slugifyTuple(tupleVal cty.Value) (cty.Value, error) {
	length := tupleVal.LengthInt()
	if length == 0 {
		return cty.ListValEmpty(cty.String), nil
	}

	out := make([]cty.Value, length)

	for i := 0; i < length; i++ {
		elem := tupleVal.Index(cty.NumberIntVal(int64(i)))
		if !elem.Type().Equals(cty.String) {
			return cty.NilVal, errListElemNotString("tm_slug", i, elem.Type())
		}
		if elem.IsNull() {
			out[i] = cty.NullVal(cty.String)
		} else {
			out[i] = cty.StringVal(slugify(elem.AsString()))
		}
	}

	return cty.ListVal(out), nil
}
