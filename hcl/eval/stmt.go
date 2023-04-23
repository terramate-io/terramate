package eval

import (
	"fmt"
	"strconv"
	"testing"

	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl/ast"
	"github.com/mineiros-io/terramate/project"
	"github.com/zclconf/go-cty/cty"
)

type (
	// Stmt represents a `var-decl` stmt.
	Stmt struct {
		LHS          Ref
		RHS          hhcl.Expression
		RHSEvaluated cty.Value
		Scope        project.Path

		IsEvaluated bool
		Special     bool

		// Origin is the *origin ref*. If it's nil, then it's the same as LHS.
		Origin Ref
	}

	// Stmts is a list of statements.
	Stmts []Stmt
)

func NewStmtHelper(t testing.TB, lhs string, rhs string) Stmts {
	lhsRef := NewRef(t, lhs)
	tmpExpr, err := ast.ParseExpression(rhs, `<test>`)
	assert.NoError(t, err)

	stmts, err := StmtsOf(project.NewPath("/"), lhsRef, lhsRef.Path, tmpExpr)
	assert.NoError(t, err)

	return stmts
}

func NewStmt(t testing.TB, lhs string, rhs string) Stmt {
	lhsRef := NewRef(t, lhs)
	rhsExpr, err := ast.ParseExpression(rhs, `<test>`)
	assert.NoError(t, err)
	return Stmt{
		Origin: lhsRef,
		LHS:    lhsRef,
		RHS:    rhsExpr,
		Scope:  project.NewPath("/"),
	}
}

// String representation of the statement.
// This function is only meant to be used in tests.
func (stmt Stmt) String() string {
	var rhs string
	if stmt.Special {
		rhs = "{} (special)"
	} else {
		rhs = string(ast.TokensForExpression(stmt.RHS).Bytes())
	}
	return fmt.Sprintf("%s = %s (defined at %s)",
		stmt.LHS,
		rhs,
		stmt.Scope)
}

func StmtsOf(scope project.Path, origin Ref, base []string, expr hhcl.Expression) (Stmts, error) {
	stmts := Stmts{}
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

				key = string(ast.TokensForExpression(v).Bytes())
				if key[0] == '"' {
					// TODO(i4k): test this
					key, _ = strconv.Unquote(key)
				}
			default:
				// TODO(i4k): test this.
				panic(errors.E("unexpected key type %T", v))
			}

			newbase[last] = key
			newStmts, err := StmtsOf(scope, origin, newbase, item.ValueExpr)
			if err != nil {
				return nil, err
			}
			stmts = append(stmts, newStmts...)
		}
	default:
		stmts = append(stmts, Stmt{
			Origin: origin,
			LHS: Ref{
				Object: origin.Object,
				Path:   newbase[0:last],
			},
			RHS:   expr,
			Scope: scope,
		})
	}

	return stmts, nil
}

func (stmts Stmts) SelectBy(ref Ref, atChild map[RefStr]Ref) (Stmts, bool) {
	found := false
	contains := Stmts{}
	isContainedBy := Stmts{}
outer:
	for _, stmt := range stmts {
		fmt.Printf("%s has %s: %t (scope %s): ", stmt.LHS, ref, stmt.LHS.Has(ref), stmt.Scope)

		if !stmt.Special {
			for _, gotRef := range atChild {
				if stmt.LHS.Has(gotRef) {
					fmt.Printf("ignored because origin %s already found in lower levels\n", gotRef)
					continue outer
				}
			}
		}

		if stmt.LHS.Has(ref) {
			fmt.Printf("contains\n")
			contains = append(contains, stmt)
			if stmt.Origin.equal(ref) || stmt.LHS.equal(ref) {
				found = true
			}
		} else {
			if found {
				fmt.Printf("found\n")
				return contains, true
			}
			if ref.Has(stmt.LHS) {
				fmt.Printf("is contained\n")
				isContainedBy = append(isContainedBy, stmt)
			} else {
				fmt.Printf("unrelated\n")
			}
		}
	}

	if len(contains) == 0 {
		return isContainedBy, false
	}

	contains = append(contains, isContainedBy...)
	return contains, false
}
