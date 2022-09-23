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
	"fmt"
	"testing"

	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/generate"
	"github.com/mineiros-io/terramate/hcl/eval"
	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
)

func TestGenerateAssert(t *testing.T) {
	testCodeGeneration(t, []testcase{
		{
			name: "no generate blocks with success assertion",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: Assert(
						Bool("assertion", true),
						Str("message", "msg"),
					),
				},
			},
			wantReport: generate.Report{},
		},
		{
			name: "assert blocks with eval failures",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: Assert(
						Expr("assertion", "unknown.ref"),
						Expr("message", "unknown.ref"),
					),
				},
			},
			wantReport: generate.Report{
				Failures: []generate.FailureResult{
					{
						Result: generate.Result{
							Dir: "/stacks/stack-1",
						},
						Error: errors.E(eval.ErrEval),
					},
					{
						Result: generate.Result{
							Dir: "/stacks/stack-2",
						},
						Error: errors.E(eval.ErrEval),
					},
				},
			},
		},
		{
			name: "no generate blocks with failed assertion",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: Assert(
						Bool("assertion", false),
						Str("message", "msg"),
					),
				},
			},
			wantReport: generate.Report{
				Failures: []generate.FailureResult{
					{
						Result: generate.Result{
							Dir: "/stacks/stack-1",
						},
						Error: errors.E(generate.ErrAssertion),
					},
					{
						Result: generate.Result{
							Dir: "/stacks/stack-2",
						},
						Error: errors.E(generate.ErrAssertion),
					},
				},
			},
		},
		{
			name: "generate blocks ignored on failed assertion",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: Doc(
						GenerateHCL(
							Labels("test.hcl"),
							Content(
								Str("stacks", "test"),
							),
						),
						GenerateFile(
							Labels("test.txt"),
							Str("content", "test"),
						),
					),
				},
				{
					path: "/stacks",
					add: Assert(
						Bool("assertion", false),
						Str("message", "msg"),
					),
				},
			},
			wantReport: generate.Report{
				Failures: []generate.FailureResult{
					{
						Result: generate.Result{
							Dir: "/stacks/stack-1",
						},
						Error: errors.E(generate.ErrAssertion),
					},
					{
						Result: generate.Result{
							Dir: "/stacks/stack-2",
						},
						Error: errors.E(generate.ErrAssertion),
					},
				},
			},
		},
		{
			name: "generates code when failed assertion is a warning",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: Doc(
						GenerateHCL(
							Labels("test.hcl"),
							Content(
								Str("stacks", "test"),
							),
						),
						GenerateFile(
							Labels("test.txt"),
							Str("content", "test"),
						),
					),
				},
				{
					path: "/stacks",
					add: Assert(
						Bool("assertion", false),
						Expr("message", `"msg"`),
						Bool("warning", true),
					),
				},
			},
			want: []generatedFile{
				{
					stack: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"test.hcl": Doc(
							Str("stacks", "test"),
						),
						"test.txt": stringer("test"),
					},
				},
				{
					stack: "/stacks/stack-2",
					files: map[string]fmt.Stringer{
						"test.hcl": Doc(
							Str("stacks", "test"),
						),
						"test.txt": stringer("test"),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     "/stacks/stack-1",
						Created: []string{"test.hcl", "test.txt"},
					},
					{
						Dir:     "/stacks/stack-2",
						Created: []string{"test.hcl", "test.txt"},
					},
				},
			},
		},
		{
			name: "failed assertions on all levels",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: Assert(
						Bool("assertion", false),
						Str("message", "msg"),
					),
				},
				{
					path: "/stacks",
					add: Assert(
						Bool("assertion", false),
						Str("message", "msg"),
					),
				},
				{
					path: "/stacks/stack-1",
					add: Assert(
						Bool("assertion", false),
						Str("message", "msg"),
					),
				},
			},
			wantReport: generate.Report{
				Failures: []generate.FailureResult{
					{
						Result: generate.Result{
							Dir: "/stacks/stack-1",
						},
						Error: errors.L(
							errors.E(generate.ErrAssertion),
							errors.E(generate.ErrAssertion),
							errors.E(generate.ErrAssertion),
						),
					},
					{
						Result: generate.Result{
							Dir: "/stacks/stack-2",
						},
						Error: errors.L(
							errors.E(generate.ErrAssertion),
							errors.E(generate.ErrAssertion),
						),
					},
				},
			},
		},
	})
}
