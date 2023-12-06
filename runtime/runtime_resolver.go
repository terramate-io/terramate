// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/project"
	"github.com/zclconf/go-cty/cty"
)

// Resolver is the runtime resolver.
// It resolves references to variables of form `terramate.<object path>`
type Resolver struct {
	terramate cty.Value
	scope     project.Path
}

// NewResolver returns a new resolver for terramate runtime variables.
func NewResolver(root *config.Root, stack *config.Stack) *Resolver {
	runtime := root.Runtime()
	var scope project.Path
	if stack != nil {
		runtime.Merge(stack.RuntimeValues(root))
		scope = stack.Dir
	} else {
		scope = project.NewPath("/")
	}
	return &Resolver{
		terramate: cty.ObjectVal(runtime),
		scope:     scope,
	}
}

// Name returns the variable name.
func (r *Resolver) Name() string { return "terramate" }

// Prevalue returns a predeclared value.
func (r *Resolver) Prevalue() cty.Value { return r.terramate }

// LookupRef lookup pending runtime variables. Not implemeneted at the moment.
func (r *Resolver) LookupRef(_ project.Path, _ eval.Ref) ([]eval.Stmts, error) {
	return []eval.Stmts{}, nil
}
