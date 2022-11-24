// Copyright 2022 Mineiros GmbH
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

package generate_test

import (
	"testing"

	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/generate"
	"github.com/mineiros-io/terramate/hcl/eval"

	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
)

func TestHCLExpressionFunc(t *testing.T) {
	// TODO(KATCIPIS): currently most behavior is tested on the genhcl pkg.
	// In the future tests could be moved here.
	testCodeGeneration(t, []testcase{
		{
			name: "tm_hcl_expression is not available",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						GenerateHCL(
							Labels("test.hcl"),
							Lets(
								Expr("expr", `tm_hcl_expression("test")`),
							),
							Content(
								Expr("value", `let.expr`),
							),
						),
						GenerateFile(
							Labels("test.txt"),
							Lets(
								Expr("content", `tm_hcl_expression("test")`),
							),
							Expr("content", "let.content"),
						),
					),
				},
			},
			wantReport: generate.Report{
				Failures: []generate.FailureResult{
					{
						Result: generate.Result{
							Dir: "/stack",
						},
						Error: errors.L(
							errors.E(eval.ErrEval),
							errors.E(eval.ErrEval),
						),
					},
				},
			},
		},
	})
}
