package globals

import (
	"sort"

	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/globals2"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/project"
	"github.com/zclconf/go-cty/cty"
)

type Resolver struct {
	tree *config.Tree
	// Scopes is a cache of scoped statements.
	Scopes map[project.Path]globals2.Stmts
}

func NewResolver(tree *config.Tree) *Resolver {
	return &Resolver{
		tree:   tree,
		Scopes: make(map[project.Path]globals2.Stmts),
	}
}

func (*Resolver) Root() string { return "global" }

func (r *Resolver) LoadStmts() (globals2.Stmts, error) {
	return r.loadStmtsAt(r.tree)
}

func (r *Resolver) LoadStmtsAt(tree *config.Tree) (globals2.Stmts, error) {
	return r.loadStmtsAt(tree)
}

func (r *Resolver) loadStmtsAt(tree *config.Tree) (globals2.Stmts, error) {
	stmts, ok := r.Scopes[tree.Dir()]
	if ok {
		return stmts, nil
	}
	for _, block := range tree.Node.Globals.AsList() {
		if len(block.Labels) > 0 && !hclsyntax.ValidIdentifier(block.Labels[0]) {
			return nil, errors.E(
				hcl.ErrTerramateSchema,
				"first global label must be a valid identifier but got %s",
				block.Labels[0],
			)
		}

		if len(block.Labels) > 0 && len(block.Attributes) == 0 {
			stmts = append(stmts, globals2.Stmt{
				Origin: globals2.Ref{
					Object: "global",
					Path:   block.Labels,
				},
				LHS: globals2.Ref{
					Object: "global",
					Path:   block.Labels,
				},
				Special: true,
				Scope:   tree.Dir(),
			})
			continue
		}

		for _, attr := range block.Attributes.SortedList() {
			origin := globals2.Ref{
				Object: "global",
				Path:   make([]string, len(block.Labels)+1),
			}
			copy(origin.Path, block.Labels)
			origin.Path[len(block.Labels)] = attr.Name
			blockStmts, err := r.stmtsOf(tree.Dir(), origin, origin.Path, attr.Expr)
			if err != nil {
				return nil, err
			}
			stmts = append(stmts, blockStmts...)
		}
	}

	// bigger refs -> smaller refs
	sort.Slice(stmts, func(i, j int) bool {
		return len(stmts[i].Origin.Path) > len(stmts[j].Origin.Path)
	})

	r.Scopes[tree.Dir()] = stmts
	return stmts, nil
}

func (r *Resolver) LookupRef(ref globals2.Ref) (globals2.Stmts, error) {
	return r.lookupStmtsAt(ref, r.tree)
}

func (r *Resolver) lookupStmtsAt(ref globals2.Ref, tree *config.Tree) (globals2.Stmts, error) {
	stmts, err := r.loadStmtsAt(tree)
	if err != nil {
		return nil, err
	}
	filtered, found := stmts.SelectBy(ref)
	if found || tree.Parent == nil {
		return filtered, nil
	}
	parentStmts, err := r.lookupStmtsAt(ref, tree.Parent)
	if err != nil {
		return nil, err
	}
	filtered = append(filtered, parentStmts...)
	return filtered, nil
}

func (r *Resolver) stmtsOf(scope project.Path, origin globals2.Ref, base []string, expr hhcl.Expression) (globals2.Stmts, error) {
	stmts := globals2.Stmts{}
	newbase := make([]string, len(base)+1)
	copy(newbase, base)
	last := len(newbase) - 1
	switch e := expr.(type) {
	case *hclsyntax.ObjectConsExpr:
		for _, item := range e.Items {
			var key string
			switch v := item.KeyExpr.(type) {
			case *hclsyntax.LiteralValueExpr:
				if !v.Val.Type().Equals(cty.String) {
					// TODO(i4k): test this.
					panic(errors.E("unexpected key type %s", v.Val.Type().FriendlyName()))
				}

				key = v.Val.AsString()
			case *hclsyntax.ObjectConsKeyExpr:
				if v.ForceNonLiteral {
					panic("TODO")
				}
				traversal := v.AsTraversal()
				if traversal == nil {
					panic("TODO")
				}
				key = traversal.RootName()
			default:
				// TODO(i4k): test this.
				panic(errors.E("unexpected key type %T", v))
			}

			newbase[last] = key
			newStmts, err := r.stmtsOf(scope, origin, newbase, item.ValueExpr)
			if err != nil {
				return nil, err
			}
			stmts = append(stmts, newStmts...)
		}
	default:
		lhs := globals2.Ref{
			Object: "global",
			Path:   newbase[0:last],
		}
		stmts = append(stmts, globals2.Stmt{
			Origin: origin,
			LHS:    lhs,
			RHS:    expr,
			Scope:  scope,
		})
	}

	return stmts, nil
}
