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

package lets

import (
	"sort"

	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl/ast"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/mapexpr"
	"github.com/mineiros-io/terramate/project"
	"github.com/zclconf/go-cty/cty"
)

const (
	// ErrRedefined indicates the lets variable is being redefined in the same
	// scope.
	ErrRedefined errors.Kind = "lets redefined"

	nsName = "let"
)

// Resolver is the lets resolver.
type Resolver struct {
	scope project.Path
	block *ast.MergedBlock

	// cache of statements.
	cached eval.Stmts
}

// NewResolver is a resolver for let.* references.
func NewResolver(scope project.Path, block *ast.MergedBlock) *Resolver {
	r := &Resolver{
		scope: scope,
		block: block,
	}
	return r
}

// Name of the resolver.
func (*Resolver) Name() string { return nsName }

// Prevalue returns predeclared lets variables.
func (r *Resolver) Prevalue() cty.Value {
	return cty.EmptyObjectVal
}

// Scope of the lets block.
func (r *Resolver) Scope() project.Path { return r.scope }

// LookupRef lookup the lets references.
func (r *Resolver) LookupRef(ref eval.Ref) (eval.Stmts, error) {
	stmts, err := r.loadStmts()
	if err != nil {
		return nil, err
	}
	var filtered eval.Stmts
	if len(ref.Path) == 0 {
		filtered = stmts
	} else {
		filtered, _ = stmts.SelectBy(ref, map[eval.RefStr]eval.Ref{})
	}

	return filtered, nil
}

func (r *Resolver) loadStmts() (eval.Stmts, error) {
	stmts := r.cached
	if stmts != nil {
		return stmts, nil
	}

	for _, varsBlock := range r.block.Blocks {
		varName := varsBlock.Labels[0]
		if _, ok := r.block.Attributes[varName]; ok {
			return nil, errors.E(
				ErrRedefined,
				"map label %s conflicts with let.%s attribute", varName, varName)
		}

		origin := eval.NewRef(nsName, varName)

		expr, err := mapexpr.NewMapExpr(varsBlock)
		if err != nil {
			return nil, errors.E(err, "failed to interpret map block")
		}

		blockStmts, err := eval.StmtsOfExpr(eval.NewInfo(r.scope, varsBlock.RawOrigins[0].Range), origin, origin.Path, expr)
		if err != nil {
			return nil, err
		}
		stmts = append(stmts, blockStmts...)
	}

	attrs := r.block.Attributes.SortedList()
	for _, attr := range attrs {
		origin := eval.NewRef(nsName, attr.Name)
		blockStmts, err := eval.StmtsOfExpr(eval.NewInfo(r.scope, attr.Range), origin, origin.Path, attr.Expr)
		if err != nil {
			return nil, err
		}
		stmts = append(stmts, blockStmts...)
	}

	// bigger refs -> smaller refs
	sort.Slice(stmts, func(i, j int) bool {
		if len(stmts[i].Origin.Path) != len(stmts[j].Origin.Path) {
			return len(stmts[i].Origin.Path) > len(stmts[j].Origin.Path)
		}
		return len(stmts[i].LHS.Path) > len(stmts[j].LHS.Path)
	})

	r.cached = stmts
	return stmts, nil
}
