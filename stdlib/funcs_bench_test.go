// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stdlib_test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/stdlib"
	"github.com/terramate-io/terramate/test"
)

func BenchmarkTmTernary(b *testing.B) {
	b.StopTimer()
	evalctx := eval.NewContext(stdlib.Functions(test.TempDir(b)))
	expr, err := ast.ParseExpression(`tm_ternary(false, tm_unknown_function(), "result")`, `bench-test`)
	assert.NoError(b, err)
	b.StartTimer()
	for n := 0; n < b.N; n++ {
		v, err := evalctx.Eval(expr)
		assert.NoError(b, err)
		if got := v.AsString(); got != "result" {
			b.Fatalf("unexpected value: %s", got)
		}
	}
}
