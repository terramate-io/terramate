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

package test

import (
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/hcl/ast"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/project"
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
