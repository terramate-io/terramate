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

	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl/ast"
	"github.com/mineiros-io/terramate/hcl/info"
	"github.com/mineiros-io/terramate/project"
	"github.com/zclconf/go-cty/cty"
)

type (
	// Stmt represents a `var-decl` stmt.
	Stmt struct {
		LHS Ref
		RHS *RHS

		Special bool

		// Origin is the *origin ref*.
		Origin Ref

		Info Info
	}

	// RHS is the right-hand side of an statement.
	// It could be an evaluated value or an expression.
	RHS struct {
		IsEvaluated bool
		Expression  hhcl.Expression
		Value       cty.Value
	}

	// Stmts is a list of statements.
	Stmts []Stmt

	// Info contains origin information for the statement.
	Info struct {
		Scope     project.Path
		DefinedAt info.Range
	}
)

// ensures RHS is a tokenizer
var _ ast.Tokenizer = NewExprRHS(nil)

// NewExprStmt creates a new stmt.
func NewExprStmt(origin Ref, lhs Ref, rhs hhcl.Expression, info Info) Stmt {
	return Stmt{
		Origin: origin,
		LHS:    lhs,
		RHS:    NewExprRHS(rhs),
		Info:   info,
	}
}

// NewValStmt creates a new stmt.
func NewValStmt(origin Ref, rhs cty.Value, info Info) Stmt {
	return NewInnerValStmt(origin, origin, rhs, info)
}

// NewInnerValStmt creates a new stmt.
func NewInnerValStmt(origin Ref, lhs Ref, rhs cty.Value, info Info) Stmt {
	return Stmt{
		Origin: origin,
		LHS:    lhs,
		RHS:    NewValRHS(rhs),
		Info:   info,
	}
}

// NewValRHS creates a new RHS for an evaluated value.
func NewValRHS(val cty.Value) *RHS {
	return &RHS{
		Value:       val,
		IsEvaluated: true,
	}
}

// NewExprRHS creates a new RHS for an unevaluated expression.
func NewExprRHS(expr hhcl.Expression) *RHS {
	return &RHS{
		Expression: expr,
	}
}

// String returns the string representation of the RHS.
func (rhs *RHS) String() string {
	if rhs.IsEvaluated {
		return ast.StringForValue(rhs.Value)
	}
	return string(ast.TokensForExpression(rhs.Expression).Bytes())
}

// Tokens tokenizes the RHS.
func (rhs *RHS) Tokens() hclwrite.Tokens {
	if rhs.IsEvaluated {
		return ast.TokensForValue(rhs.Value)
	}
	return ast.TokensForExpression(rhs.Expression)
}

// NewExtendStmt returns a statement that extend existent object accessors.
func NewExtendStmt(origin Ref, info Info) Stmt {
	return Stmt{
		Origin:  origin,
		LHS:     origin,
		Special: true,
		Info:    info,
	}
}

// String representation of the statement.
// This function is only meant to be used in tests.
func (stmt Stmt) String() string {
	var rhs string
	if stmt.Special {
		rhs = "{} (Extend)"
	} else if stmt.RHS.IsEvaluated {
		rhs = string(ast.TokensForValue(stmt.RHS.Value).Bytes())
	} else {
		rhs = string(ast.TokensForExpression(stmt.RHS.Expression).Bytes())
	}
	return fmt.Sprintf("%s = %s (scope=%s, definedAt=%s)",
		stmt.LHS,
		rhs,
		stmt.Info.Scope,
		stmt.Info.DefinedAt,
	)
}

// StmtsOfValue returns all inners statements of the provided value.
func StmtsOfValue(info Info, origin Ref, base []string, val cty.Value) Stmts {
	stmts := Stmts{}
	if !val.Type().IsObjectType() {
		stmts = append(stmts, NewInnerValStmt(
			origin,
			NewRef(origin.Object, base...),
			val,
			info,
		))
		return stmts
	}

	newbase := make([]string, len(base)+1)
	copy(newbase, base)
	last := len(newbase) - 1
	objMap := val.AsValueMap()
	for key, value := range objMap {
		newbase[last] = key
		stmts = append(stmts, StmtsOfValue(info, origin, newbase, value)...)
	}
	return stmts
}

// StmtsOfExpr returns all statements of the expr.
func StmtsOfExpr(info Info, origin Ref, base []string, expr hhcl.Expression) (Stmts, error) {
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
			newStmts, err := StmtsOfExpr(info, origin, newbase, item.ValueExpr)
			if err != nil {
				return nil, err
			}
			stmts = append(stmts, newStmts...)
		}
	default:
		stmts = append(stmts, NewExprStmt(
			origin,
			NewRef(origin.Object, newbase[0:last]...),
			expr,
			info,
		))
	}

	return stmts, nil
}

// SelectBy selects the statements related to ref.
func (stmts Stmts) SelectBy(ref Ref, atChild map[RefStr]Ref) (Stmts, bool) {
	found := false
	contains := Stmts{}
	isContainedBy := Stmts{}
outer:
	for _, stmt := range stmts {
		if stmt.LHS.Object != ref.Object {
			// unrelated
			continue
		}

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

// NewInfo returns a new Info.
func NewInfo(scope project.Path, rng info.Range) Info {
	return Info{
		Scope:     scope,
		DefinedAt: rng,
	}
}

func newBuiltinInfo(scope project.Path) Info {
	return Info{
		Scope: scope,
	}
}
