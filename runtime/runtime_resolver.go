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
	"github.com/zclconf/go-cty/cty"
)

type Resolver struct {
	terramate cty.Value
}

func NewResolver(root *config.Root, stack *config.Stack) *Resolver {
	runtime := root.Runtime()
	if stack != nil {
		runtime.Merge(stack.RuntimeValues(root))
	}
	return &Resolver{
		terramate: cty.ObjectVal(runtime),
	}
}

func (r *Resolver) Root() string { return "terramate" }

func (r *Resolver) Prevalue() cty.Value { return r.terramate }

func (r *Resolver) LoadStmts() (eval.Stmts, error) {
	return eval.Stmts{}, nil
}

func (r *Resolver) LookupRef(ref eval.Ref) (eval.Stmts, error) {
	return eval.Stmts{}, nil
}
