// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stdlib

import (
	"errors"
	"fmt"
	"strings"

	"github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/hcl/v2/ext/customdecode"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

// TryFunc implements the `tm_try()` function.
func TryFunc() function.Function {
	return function.New(&function.Spec{
		VarParam: &function.Parameter{
			Name: "expressions",
			Type: customdecode.ExpressionClosureType,
		},
		Type: function.StaticReturnType(cty.DynamicPseudoType),
		Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
			return try(args)
		},
	})
}

func try(args []cty.Value) (cty.Value, error) {
	if len(args) == 0 {
		return cty.NilVal, errors.New("at least one argument is required")
	}

	// We'll collect up all of the diagnostics we encounter along the way
	// and report them all if none of the expressions succeed, so that the
	// user might get some hints on how to make at least one succeed.
	var diags hcl.Diagnostics
	for _, arg := range args {
		closure := customdecode.ExpressionClosureFromVal(arg)
		v, moreDiags := closure.Value()
		diags = append(diags, moreDiags...)
		if moreDiags.HasErrors() {
			continue
		}
		return v, nil // ignore any accumulated diagnostics if one succeeds
	}

	// If we fall out here then none of the expressions succeeded, and so
	// we must have at least one diagnostic and we'll return all of them
	// so that the user can see the errors related to whichever one they
	// were expecting to have succeeded in this case.
	//
	// Because our function must return a single error value rather than
	// diagnostics, we'll construct a suitable error message string
	// that will make sense in the context of the function call failure
	// diagnostic HCL will eventually wrap this in.
	var buf strings.Builder
	buf.WriteString("no expression succeeded:\n")
	for _, diag := range diags {
		if diag.Subject != nil {
			buf.WriteString(fmt.Sprintf("- %s (at %s)\n  %s\n", diag.Summary, diag.Subject, diag.Detail))
		} else {
			buf.WriteString(fmt.Sprintf("- %s\n  %s\n", diag.Summary, diag.Detail))
		}
	}
	buf.WriteString("\nAt least one expression must produce a successful result")
	return cty.NilVal, errors.New(buf.String())
}
