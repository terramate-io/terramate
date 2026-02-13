// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package generate_test

import (
	"fmt"
	"testing"

	genreport "github.com/terramate-io/terramate/generate/report"
	"github.com/terramate-io/terramate/project"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
)

func TestGenerateBundle(t *testing.T) {
	t.Parallel()

	testCodeGeneration(t, []testcase{
		{
			name: "generate bundle",
			layout: []string{
				"s:stacks/stack-1",
			},
			configs: []hclconfig{
				{
					path: "/components/example.com/my-component/v1",
					add: Doc(
						Block("define",
							Labels("component", "metadata"),
							Str("class", "my-component"),
							Str("name", "my-name"),
							Str("version", "1.2.3"),
							Str("description", "My component"),
						),

						Block("define",
							Labels("component", "input", "test"),
							Str("prompt", "Test value"),
							Str("description", "Test value"),
						),

						Block("generate_hcl",
							Labels("main.tf"),
							Block("content",
								Expr("value", "component.input.test.value"),
							),
						),
					),
				},
				{
					path: "/bundles/example.com/my-bundle/v1",
					add: Doc(
						Block("define",
							Labels("bundle", "metadata"),
							Str("class", "my-bundle"),
							Str("name", "my-name"),
							Str("version", "1.2.3"),
							Str("description", "My bundle"),
						),

						Block("define",
							Labels("bundle", "input", "test"),
							Str("prompt", "Test value"),
							Str("description", "Test value"),
						),

						Block("define",
							Labels("bundle", "stack", "my-stack"),
							Block("metadata",
								Str("path", "my-stack"),
								Str("name", "my-stack"),
							),
							Block("component",
								Labels("my-comp"),
								Str("source", "/components/example.com/my-component/v1"),
								Block("inputs",
									Expr("test", "bundle.input.test.value"),
								),
							),
						),
					),
				},
				{
					path: "/stacks/stack-1",
					add: Block("bundle",
						Labels("name"),
						Str("source", "/bundles/example.com/my-bundle/v1"),
						Block("inputs",
							Str("test", `some_input`),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack-1/my-stack",
					files: map[string]fmt.Stringer{
						"component_my-comp_main.tf": stringer(`value = "some_input"`),
					},
				},
			},
			wantReport: genreport.Report{
				Successes: []genreport.Result{
					{
						Dir:     project.NewPath("/stacks/stack-1/my-stack"),
						Created: []string{"component_my-comp_main.tf", "stack.tm.hcl"},
					},
				},
			},
		},
		{
			name: "generate bundle with conditions",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
				"s:stacks/stack-3",
			},
			configs: []hclconfig{
				{
					path: "/components/example.com/my-component/v1",
					add: Doc(
						Block("define",
							Labels("component", "metadata"),
							Str("class", "my-component"),
							Str("name", "my-name"),
							Str("version", "1.2.3"),
							Str("description", "My component"),
						),

						Block("define",
							Labels("component", "input", "test"),
							Str("prompt", "Test value"),
							Str("description", "Test value"),
						),

						Block("generate_hcl",
							Labels("main.tf"),
							Block("content",
								Expr("value", "component.input.test.value"),
							),
						),
					),
				},
				{
					path: "/bundles/example.com/my-bundle/v1",
					add: Doc(
						Block("define",
							Labels("bundle", "metadata"),
							Str("class", "my-bundle"),
							Str("name", "my-name"),
							Str("version", "1.2.3"),
							Str("description", "My bundle"),
						),
						Block("define",
							Labels("bundle", "input", "test"),
						),
						Block("define",
							Labels("bundle", "input", "with_stack"),
							Bool("default", false),
						),
						Block("define",
							Labels("bundle", "input", "with_component"),
							Bool("default", true),
						),
						Block("define",
							Labels("bundle", "stack", "my-stack"),
							Expr("condition", "bundle.input.with_stack.value"),
							Block("metadata",
								Str("path", "my-stack"),
								Str("name", "my-stack"),
							),
							Block("component",
								Labels("my-comp"),
								Expr("condition", "bundle.input.with_component.value"),
								Str("source", "/components/example.com/my-component/v1"),
								Block("inputs",
									Expr("test", "bundle.input.test.value"),
								),
							),
						),
					),
				},
				{
					path: "/stacks/stack-1",
					add: Block("bundle",
						Labels("name"),
						Str("source", "/bundles/example.com/my-bundle/v1"),
						Block("inputs",
							Str("test", `some_input1`),
						),
					),
				},
				{
					path: "/stacks/stack-2",
					add: Block("bundle",
						Labels("name"),
						Str("source", "/bundles/example.com/my-bundle/v1"),
						Block("inputs",
							Str("test", `some_input2`),
							Bool("with_stack", true),
						),
					),
				},
				{
					path: "/stacks/stack-3",
					add: Block("bundle",
						Labels("name"),
						Str("source", "/bundles/example.com/my-bundle/v1"),
						Block("inputs",
							Str("test", `some_input2`),
							Bool("with_stack", true),
							Bool("with_component", false),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack-2/my-stack",
					files: map[string]fmt.Stringer{
						"component_my-comp_main.tf": stringer(`value = "some_input2"`),
					},
				},
			},
			wantReport: genreport.Report{
				Successes: []genreport.Result{
					{
						Dir:     project.NewPath("/stacks/stack-2/my-stack"),
						Created: []string{"component_my-comp_main.tf", "stack.tm.hcl"},
					},
					{
						Dir:     project.NewPath("/stacks/stack-3/my-stack"),
						Created: []string{"stack.tm.hcl"},
					},
				},
			},
		},
		{
			name: "generate bundle stack with absolute path",
			layout: []string{
				"s:stacks/stack-1",
			},
			configs: []hclconfig{
				{
					path: "/components/example.com/my-component/v1",
					add: Doc(
						Block("define",
							Labels("component", "metadata"),
							Str("class", "my-component"),
							Str("name", "my-name"),
							Str("version", "1.2.3"),
							Str("description", "My component"),
						),

						Block("define",
							Labels("component", "input", "test"),
							Str("prompt", "Test value"),
							Str("description", "Test value"),
						),

						Block("generate_hcl",
							Labels("main.tf"),
							Block("content",
								Expr("value", "component.input.test.value"),
							),
						),
					),
				},
				{
					path: "/bundles/example.com/my-bundle/v1",
					add: Doc(
						Block("define",
							Labels("bundle", "metadata"),
							Str("class", "my-bundle"),
							Str("name", "my-name"),
							Str("version", "1.2.3"),
							Str("description", "My bundle"),
						),
						Block("define",
							Labels("bundle", "input", "test"),
						),
						Block("define",
							Labels("bundle", "stack", "my-stack"),
							Block("metadata",
								Str("path", "/generated_stacks/my-stack"),
								Str("name", "my-stack"),
							),
							Block("component",
								Labels("my-comp"),
								Str("source", "/components/example.com/my-component/v1"),
								Block("inputs",
									Expr("test", "bundle.input.test.value"),
								),
							),
						),
					),
				},
				{
					path: "/stacks/stack-1",
					add: Block("bundle",
						Labels("name"),
						Str("source", "/bundles/example.com/my-bundle/v1"),
						Block("inputs",
							Str("test", `some_input1`),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/generated_stacks/my-stack",
					files: map[string]fmt.Stringer{
						"component_my-comp_main.tf": stringer(`value = "some_input1"`),
					},
				},
			},
			wantReport: genreport.Report{
				Successes: []genreport.Result{
					{
						Dir:     project.NewPath("/generated_stacks/my-stack"),
						Created: []string{"component_my-comp_main.tf", "stack.tm.hcl"},
					},
				},
			},
		},
		{
			name: "don't generate files in .terramate/ for bundle references",
			layout: []string{
				`s:.terramate/stack:id=s1;tags=["genstack"]`,
				"s:other",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: GenerateHCL(
						Labels("file.hcl"),
						Content(
							Str("data", "data"),
						),
						Expr("condition", `tm_contains(terramate.stack.tags, "genstack")`),
					),
				},
				{
					path:     "/.terramate/stack",
					filename: "bundle_def.tm.hcl",
					add: Doc(
						Block("define",
							Labels("bundle", "metadata"),
							Str("class", "my-bundle"),
							Str("name", "my-name"),
							Str("version", "1.2.3"),
							Str("description", "My bundle"),
						),

						Block("define",
							Labels("bundle", "input", "test"),
							Str("prompt", "Test value"),
							Str("description", "Test value"),
						),
					),
				},
				{
					path:     "/other",
					filename: "bundle_use.tm.hcl",
					add: Block("bundle",
						Labels("name"),
						Str("source", "/.terramate/stack"),
						Block("inputs",
							Str("test", `some_input`),
						),
					),
				},
			},
			wantReport: genreport.Report{},
		},
	})
}
