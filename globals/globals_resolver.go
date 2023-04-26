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

const ErrRedefined errors.Kind = "global redefined"

const nsName = "global"

type Resolver struct {
	tree *config.Tree

	override map[eval.RefStr]eval.Stmt
	// Scopes is a cache of scoped statements.
	Scopes map[project.Path]eval.Stmts
}

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

func (*Resolver) Root() string { return nsName }

func (r *Resolver) Prevalue() cty.Value {
	return cty.EmptyObjectVal
}

func (r *Resolver) LoadStmtsAt(tree *config.Tree) (eval.Stmts, error) {
	return r.loadStmtsAt(tree)
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
			stmts = append(stmts, eval.Stmt{
				Origin: eval.Ref{
					Object: nsName,
					Path:   block.Labels,
				},
				LHS: eval.Ref{
					Object: nsName,
					Path:   block.Labels,
				},
				Special: true,
				Info:    eval.NewInfo(scope, block.RawOrigins[0].Range),
			})
		}

		for _, varsBlock := range block.Blocks {
			varName := varsBlock.Labels[0]
			if _, ok := block.Attributes[varName]; ok {
				return nil, errors.E(
					ErrRedefined,
					"map label %s conflicts with global.%s attribute", varName, varName)
			}

			definedAt := varsBlock.RawOrigins[0].Range

			origin := eval.Ref{
				Object: nsName,
				Path:   make([]string, len(block.Labels)+1),
			}

			copy(origin.Path, block.Labels)
			origin.Path[len(block.Labels)] = varName

			if _, ok := overrideMap[origin.AsKey()]; ok {
				continue
			}

			expr, err := mapexpr.NewMapExpr(varsBlock)
			if err != nil {
				return nil, errors.E(err, "failed to interpret map block")
			}

			info := eval.NewInfo(tree.Dir(), definedAt)
			blockStmts, err := eval.StmtsOf(info, origin, origin.Path, expr)
			if err != nil {
				return nil, err
			}
			stmts = append(stmts, blockStmts...)
		}

		for _, attr := range attrs {
			origin := eval.Ref{
				Object: nsName,
				Path:   make([]string, len(block.Labels)+1),
			}
			copy(origin.Path, block.Labels)
			origin.Path[len(block.Labels)] = attr.Name

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

func (r *Resolver) LookupRef(ref eval.Ref) (eval.Stmts, error) {
	return r.lookupStmtsAt(ref, r.tree, map[eval.RefStr]eval.Ref{})
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
