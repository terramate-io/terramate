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

// Package dynexpr provides helper functions for dealing with dynamically
// generated expressions.
//
// Those helpers are needed because Terramate has a very low-level Partial
// Evaluation feature which works at the token level. It loads the tokens for
// each expression from the file range defined in the expression but the
// resulting evaluated expression tokens doesn't match the original tokens
// anymore and in the case of recursive evaluation (tm_ternary, etc) the
// Hashicorp library doesn't provide any place to obtain the evaluated tokens.
// So the hack here is to inject them inside the expr.Range.Filename and extract
// the tokens when needed, but the injection *must* be cleaned up before returning
// to the user.
package dynexpr

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl/dynexpr/dynrange"
)

// ParseExpressionBytes parses the exprBytes into a hcl.Expression and stores
// the original tokens inside the expression.Range().Filename. For details
// about this craziness, see the [eval.TokensForExpression()] function.
func ParseExpressionBytes(exprBytes []byte) (hcl.Expression, error) {
	expr, diags := hclsyntax.ParseExpression(
		exprBytes,
		dynrange.WrapExprBytes(exprBytes),
		hcl.Pos{
			Line:   1,
			Column: 1,
			Byte:   0,
		})

	if diags.HasErrors() {
		return nil, errors.E(diags, "parsing expression bytes")
	}
	return expr, nil
}
