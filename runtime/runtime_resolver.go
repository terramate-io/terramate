// Copyright 2023 Mineiros GmbH
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

package runtime

import (
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/project"
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

// Scope of the runtime resolver.
func (r *Resolver) Scope() project.Path { return project.NewPath("/") }

// Name returns the variable name.
func (r *Resolver) Name() string { return "terramate" }

// Prevalue returns a predeclared value.
func (r *Resolver) Prevalue() cty.Value { return r.terramate }

// LookupRef lookup pending runtime variables. Not implemeneted at the moment.
func (r *Resolver) LookupRef(ref eval.Ref) (eval.Stmts, error) {
	return eval.Stmts{}, nil
}
