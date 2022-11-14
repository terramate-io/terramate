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

	"github.com/mineiros-io/terramate/generate"
	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
)

func TestGenerateVendor(t *testing.T) {
	t.Parallel()

	testCodeGeneration(t, []testcase{
		{
			name: "tm_vendor path is relative to generate blocks label",
			layout: []string{
				"s:stacks/stack",
				"s:stacks/stack/substack",
			},
			vendorDir: "/vendor",
			configs: []hclconfig{
				{
					path: "/stacks",
					add: Doc(
						GenerateHCL(
							Labels("file.hcl"),
							Content(
								Expr("vendor", `tm_vendor("github.com/mineiros-io/terramate?ref=v1")`),
							),
						),
						GenerateFile(
							Labels("file.txt"),
							Expr("content", `tm_vendor("github.com/mineiros-io/terramate?ref=v2")`),
						),
						GenerateHCL(
							Labels("dir/file.hcl"),
							Content(
								Expr("vendor", `tm_vendor("github.com/mineiros-io/terramate?ref=v3")`),
							),
						),
						GenerateFile(
							Labels("dir/file.txt"),
							Expr("content", `tm_vendor("github.com/mineiros-io/terramate?ref=v4")`),
						),
					),
				},
			},
			want: []generatedFile{
				{
					stack: "/stacks/stack",
					files: map[string]fmt.Stringer{
						"dir/file.hcl": Doc(
							Str("vendor", "../../../vendor/github.com/mineiros-io/terramate/v3"),
						),
						"dir/file.txt": stringer("../../../vendor/github.com/mineiros-io/terramate/v4"),
						"file.hcl": Doc(
							Str("vendor", "../../vendor/github.com/mineiros-io/terramate/v1"),
						),
						"file.txt": stringer("../../vendor/github.com/mineiros-io/terramate/v2"),
					},
				},
				{
					stack: "/stacks/stack/substack",
					files: map[string]fmt.Stringer{
						"dir/file.hcl": Doc(
							Str("vendor", "../../../../vendor/github.com/mineiros-io/terramate/v3"),
						),
						"dir/file.txt": stringer("../../../../vendor/github.com/mineiros-io/terramate/v4"),
						"file.hcl": Doc(
							Str("vendor", "../../../vendor/github.com/mineiros-io/terramate/v1"),
						),
						"file.txt": stringer("../../../vendor/github.com/mineiros-io/terramate/v2"),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir: "/stacks/stack",
						Created: []string{
							"dir/file.hcl",
							"dir/file.txt",
							"file.hcl",
							"file.txt",
						},
					},
					{
						Dir: "/stacks/stack/substack",
						Created: []string{
							"dir/file.hcl",
							"dir/file.txt",
							"file.hcl",
							"file.txt",
						},
					},
				},
			},
		},
	})
}
