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
	"github.com/mineiros-io/terramate/hcl/info"
	"github.com/mineiros-io/terramate/project"
	"github.com/zclconf/go-cty/cty"
)

type (
	// Stmt represents a `var-decl` stmt.
	Stmt struct {
		LHS          Ref
		RHS          hhcl.Expression
		RHSEvaluated cty.Value

		IsEvaluated bool
		Special     bool

		// Origin is the *origin ref*. If it's nil, then it's the same as LHS.
		Origin Ref

		Info Info
	}

	// Stmts is a list of statements.
	Stmts []Stmt

	Info struct {
		Scope     project.Path
		DefinedAt info.Range
	}
)

func NewStmtHelper(t testing.TB, lhs string, rhs string) Stmts {
	lhsRef := NewRef(t, lhs)
	tmpExpr, err := ast.ParseExpression(rhs, `<test>`)
	assert.NoError(t, err)

	stmts, err := StmtsOf(newSameInfo("/"), lhsRef, lhsRef.Path, tmpExpr)
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
		Info:   newSameInfo("/"),
	}
}

// String representation of the statement.
// This function is only meant to be used in tests.
func (stmt Stmt) String() string {
	var rhs string
	if stmt.Special {
		rhs = "{} (special)"
	} else if stmt.RHS == nil {
		rhs = string(ast.TokensForValue(stmt.RHSEvaluated).Bytes())
	} else {
		rhs = string(ast.TokensForExpression(stmt.RHS).Bytes())
	}
	return fmt.Sprintf("%s = %s (scope=%s, definedAt=%s)",
		stmt.LHS,
		rhs,
		stmt.Info.Scope,
		stmt.Info.DefinedAt,
	)
}

func StmtsOf(info Info, origin Ref, base []string, expr hhcl.Expression) (Stmts, error) {
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
			newStmts, err := StmtsOf(info, origin, newbase, item.ValueExpr)
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
			RHS:  expr,
			Info: info,
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
		if !stmt.Special {
			for _, gotRef := range atChild {
				if stmt.LHS.Has(gotRef) {
					continue outer
				}
			}
		}

		if stmt.LHS.Has(ref) {
			contains = append(contains, stmt)
			if stmt.Origin.equal(ref) || stmt.LHS.equal(ref) {
				found = true
			}
		} else {
			if found {
				return contains, true
			}
			if ref.Has(stmt.LHS) {
				isContainedBy = append(isContainedBy, stmt)
			}
		}
	}

	if len(contains) == 0 {
		return isContainedBy, false
	}

	contains = append(contains, isContainedBy...)
	return contains, false
}

func newSameInfo(path string) Info {
	return Info{
		Scope: project.NewPath(path),
	}
}

func NewInfo(scope project.Path, rng info.Range) Info {
	return Info{
		Scope:     scope,
		DefinedAt: rng,
	}
}
