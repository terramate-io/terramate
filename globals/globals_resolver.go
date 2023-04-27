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

package globals

import (
	"sort"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/mapexpr"
	"github.com/mineiros-io/terramate/project"
	"github.com/zclconf/go-cty/cty"
)

// ErrRedefined indicates the global is redefined.
const ErrRedefined errors.Kind = "global redefined"

const nsName = "global"

// Resolver is the globals resolver.
type Resolver struct {
	tree *config.Tree

	override map[eval.RefStr]eval.Stmt
	// Scopes is a cache of scoped statements.
	Scopes map[project.Path]eval.Stmts
}

// NewResolver creates a new globals resolver.
func NewResolver(tree *config.Tree, overrides ...eval.Stmt) *Resolver {
	r := &Resolver{
		tree:     tree,
		Scopes:   make(map[project.Path]eval.Stmts),
		override: make(map[eval.RefStr]eval.Stmt),
	}

	for _, override := range overrides {
		r.override[override.LHS.AsKey()] = override
	}

	return r
}

// Scope of the resolver.
func (r *Resolver) Scope() project.Path { return r.tree.Dir() }

// Name of the variable.
func (*Resolver) Name() string { return nsName }

// Prevalue is the predeclared globals.
func (r *Resolver) Prevalue() cty.Value {
	return cty.EmptyObjectVal
}

// LookupRef lookups global references.
func (r *Resolver) LookupRef(ref eval.Ref) (eval.Stmts, error) {
	return r.lookupStmtsAt(ref, r.tree, map[eval.RefStr]eval.Ref{})
}

func (r *Resolver) loadStmtsAt(tree *config.Tree) (eval.Stmts, error) {
	stmts, ok := r.Scopes[tree.Dir()]
	if ok {
		return stmts, nil
	}

	overrideMap := map[eval.RefStr]struct{}{}

	for _, override := range r.override {
		stmts = append(stmts, override)
		overrideMap[override.Origin.AsKey()] = struct{}{}
	}

	for _, block := range tree.Node.Globals.AsList() {
		if len(block.Labels) > 0 && !hclsyntax.ValidIdentifier(block.Labels[0]) {
			return nil, errors.E(
				hcl.ErrTerramateSchema,
				"first global label must be a valid identifier but got %s",
				block.Labels[0],
			)
		}

		attrs := block.Attributes.SortedList()
		if len(block.Labels) > 0 {
			scope := tree.Dir()
			stmts = append(stmts, eval.NewExtendStmt(
				eval.NewRef(nsName, block.Labels...),
				eval.NewInfo(scope, block.RawOrigins[0].Range),
			))
		}

		for _, varsBlock := range block.Blocks {
			varName := varsBlock.Labels[0]
			if _, ok := block.Attributes[varName]; ok {
				return nil, errors.E(
					ErrRedefined,
					"map label %s conflicts with global.%s attribute", varName, varName)
			}

			origin := eval.NewRef(nsName, block.Labels...)
			origin.Path = append(origin.Path, varName)

			if _, ok := overrideMap[origin.AsKey()]; ok {
				continue
			}

			expr, err := mapexpr.NewMapExpr(varsBlock)
			if err != nil {
				return nil, errors.E(err, "failed to interpret map block")
			}

			info := eval.NewInfo(tree.Dir(), varsBlock.RawOrigins[0].Range)
			blockStmts, err := eval.StmtsOf(info, origin, origin.Path, expr)
			if err != nil {
				return nil, err
			}
			stmts = append(stmts, blockStmts...)
		}

		for _, attr := range attrs {
			origin := eval.NewRef(nsName, block.Labels...)
			origin.Path = append(origin.Path, attr.Name)

			if _, ok := overrideMap[origin.AsKey()]; ok {
				continue
			}

			info := eval.NewInfo(tree.Dir(), attr.Range)
			blockStmts, err := eval.StmtsOf(info, origin, origin.Path, attr.Expr)
			if err != nil {
				return nil, err
			}
			stmts = append(stmts, blockStmts...)
		}
	}

	// bigger refs -> smaller refs
	sort.Slice(stmts, func(i, j int) bool {
		if len(stmts[i].Origin.Path) != len(stmts[j].Origin.Path) {
			return len(stmts[i].Origin.Path) > len(stmts[j].Origin.Path)
		}
		return len(stmts[i].LHS.Path) > len(stmts[j].LHS.Path)
	})

	r.Scopes[tree.Dir()] = stmts
	return stmts, nil
}

func (r *Resolver) lookupStmtsAt(ref eval.Ref, tree *config.Tree, origins map[eval.RefStr]eval.Ref) (stmts eval.Stmts, err error) {
	stmts, err = r.loadStmtsAt(tree)
	if err != nil {
		return nil, err
	}

	filtered, found := stmts.SelectBy(ref, origins)
	for _, s := range filtered {
		if !s.Special {
			origins[s.Origin.AsKey()] = s.Origin
		}
	}

	if found || tree.Parent == nil {
		return filtered, nil
	}

	parent := tree.NonEmptyGlobalsParent()
	if parent == nil {
		return filtered, nil
	}

	parentStmts, err := r.lookupStmtsAt(ref, parent, origins)
	if err != nil {
		return nil, err
	}
	filtered = append(filtered, parentStmts...)
	return filtered, nil
}
