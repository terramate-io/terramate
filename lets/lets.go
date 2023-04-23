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

const ErrRedefined errors.Kind = "lets redefined"

const nsName = "let"

type Resolver struct {
	scope project.Path
	block *ast.MergedBlock

	// cache of statements.
	cached eval.Stmts
}

func NewResolver(scope project.Path, block *ast.MergedBlock) *Resolver {
	r := &Resolver{
		scope: scope,
		block: block,
	}
	return r
}

func (*Resolver) Root() string { return nsName }

func (r *Resolver) Prevalue() cty.Value {
	return cty.EmptyObjectVal
}

func (r *Resolver) LoadStmts() (eval.Stmts, error) {
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

		origin := eval.Ref{
			Object: nsName,
			Path:   make([]string, 1),
		}

		origin.Path[0] = varName

		expr, err := mapexpr.NewMapExpr(varsBlock)
		if err != nil {
			return nil, errors.E(err, "failed to interpret map block")
		}

		blockStmts, err := eval.StmtsOf(r.scope, origin, origin.Path, expr)
		if err != nil {
			return nil, err
		}
		stmts = append(stmts, blockStmts...)
	}

	attrs := r.block.Attributes.SortedList()
	for _, attr := range attrs {
		origin := eval.Ref{
			Object: nsName,
			Path:   make([]string, 1),
		}

		origin.Path[0] = attr.Name
		blockStmts, err := eval.StmtsOf(r.scope, origin, origin.Path, attr.Expr)
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

func (r *Resolver) LookupRef(ref eval.Ref) (eval.Stmts, error) {
	stmts, err := r.LoadStmts()
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
