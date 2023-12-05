// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package lets

import (
	"sort"

	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/mapexpr"
	"github.com/terramate-io/terramate/project"
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
func NewResolver(block *ast.MergedBlock) *Resolver {
	r := &Resolver{
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

// LookupRef lookup the lets references.
func (r *Resolver) LookupRef(_ project.Path, ref eval.Ref) ([]eval.Stmts, error) {
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

	return []eval.Stmts{filtered}, nil
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
