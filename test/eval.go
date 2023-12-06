// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package test

import (
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/project"
)

// NewRef returns a new variable reference for testing purposes.
// It only allows basic dotted accessors.
func NewRef(t testing.TB, varname string) eval.Ref {
	t.Helper()
	paths := strings.Split(varname, ".")
	if strings.Contains(varname, "[") {
		t.Fatalf("invalid testing reference: %s", varname)
	}
	return eval.Ref{
		Object: paths[0],
		Path:   paths[1:],
	}
}

// NewStmtFrom is a testing purspose method that initializes a Stmt.
func NewStmtFrom(t testing.TB, lhs string, rhs string) eval.Stmts {
	lhsRef := NewRef(t, lhs)
	tmpExpr, err := ast.ParseExpression(rhs, `<test>`)
	assert.NoError(t, err)

	stmts, err := eval.StmtsOfExpr(newSameInfo("/"), lhsRef, lhsRef.Path, tmpExpr)
	assert.NoError(t, err)

	return stmts
}

// NewStmt is a testing purspose method that initializes a Stmt.
func NewStmt(t testing.TB, lhs string, rhs string) eval.Stmt {
	lhsRef := NewRef(t, lhs)
	rhsExpr, err := ast.ParseExpression(rhs, `<test>`)
	assert.NoError(t, err)
	return eval.Stmt{
		Origin: lhsRef,
		LHS:    lhsRef,
		RHS:    eval.NewExprRHS(rhsExpr),
		Info:   newSameInfo("/"),
	}
}

func newSameInfo(path string) eval.Info {
	return eval.Info{
		Scope: project.NewPath(path),
	}
}
