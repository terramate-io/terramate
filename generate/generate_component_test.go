// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package generate_test

import (
	"fmt"
	"testing"

	"github.com/terramate-io/terramate/errors"
	genreport "github.com/terramate-io/terramate/generate/report"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/project"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
)

func TestGenerateComponent(t *testing.T) {
	t.Parallel()

	testCodeGeneration(t, []testcase{
		{
			name: "generate component",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
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
					path: "/stacks/stack-1",
					add: Block("component",
						Labels("name"),
						Str("source", "/components/example.com/my-component/v1"),
						Expr("inputs", `{
							"test" = terramate.stack.name
						}`),
					),
				},
				{
					path: "/stacks/stack-2",
					add: Block("component",
						Labels("name"),
						Str("source", "/components/example.com/my-component/v1"),
						Expr("inputs", `{
							"test" = terramate.stack.name
						}`),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"component_name_main.tf": stringer(`value = "stack-1"`),
					},
				},
				{
					dir: "/stacks/stack-2",
					files: map[string]fmt.Stringer{
						"component_name_main.tf": stringer(`value = "stack-2"`),
					},
				},
			},
			wantReport: genreport.Report{
				Successes: []genreport.Result{
					{
						Dir:     project.NewPath("/stacks/stack-1"),
						Created: []string{"component_name_main.tf"},
					},
					{
						Dir:     project.NewPath("/stacks/stack-2"),
						Created: []string{"component_name_main.tf"},
					},
				},
			},
		},
		{
			name: "generate component with unknown input silently ignored",
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
					path: "/stacks/stack-1",
					add: Block("component",
						Labels("name"),
						Str("source", "/components/example.com/my-component/v1"),
						Expr("inputs", `{
							"test" = "hello"
							"unknown" = "should be ignored"
						}`),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"component_name_main.tf": stringer(`value = "hello"`),
					},
				},
			},
			wantReport: genreport.Report{
				Successes: []genreport.Result{
					{
						Dir:     project.NewPath("/stacks/stack-1"),
						Created: []string{"component_name_main.tf"},
					},
				},
			},
		},
		{
			name: "generate component with unknown input in inputs block returns error",
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
					path: "/stacks/stack-1",
					add: Doc(
						Block("component",
							Labels("name"),
							Str("source", "/components/example.com/my-component/v1"),
							Block("inputs",
								Str("test", "hello"),
								Str("unknown", "should produce error"),
							),
						),
					),
				},
			},
			wantReport: genreport.Report{
				Failures: []genreport.FailureResult{
					{
						Result: genreport.Result{
							Dir: project.NewPath("/stacks/stack-1"),
						},
						Error: errors.E(hcl.ErrTerramateSchema),
					},
				},
			},
		},
		{
			name: "generate component with default input values",
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
						),
						Block("define",
							Labels("component", "input", "a"),
							Str("default", "a-default"),
							Str("prompt", "Enter a value for a"),
						),
						Block("define",
							Labels("component", "input", "b"),
							Str("prompt", "Enter a value for b"),
							// no default
						),
						Block("generate_hcl",
							Labels("main.tf"),
							Block("content",
								Expr("a", `component.input.a.value`),
								Expr("b", `component.input.b.value`),
							),
						),
					),
				},
				{
					path: "/stacks/stack-1",
					add: Block("component",
						Labels("name"),
						Str("source", "/components/example.com/my-component/v1"),
						Expr("inputs", `{
							b = "b value"
						}`),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"component_name_main.tf": stringer(`a = "a-default"
b = "b value"`),
					},
				},
			},
			wantReport: genreport.Report{
				Successes: []genreport.Result{
					{
						Dir:     project.NewPath("/stacks/stack-1"),
						Created: []string{"component_name_main.tf"},
					},
				},
			},
		},
		{
			name: "generate component with default value coming from required input value",
			layout: []string{
				"s:stacks/stack-1",
			},
			configs: []hclconfig{
				{
					path: "/components/example.com/my-component/v1",
					add: Doc(
						Block("define",
							Labels("component", "metadata"),
							Str("class", "component-class"),
							Str("name", "my-name"),
							Str("version", "1.2.3"),
							Str("description", "My component"),
						),
						Block("define",
							Labels("component", "input", "a"),
							Str("prompt", "Enter a value"),
						),
						Block("define",
							Labels("component", "input", "b"),
							Str("prompt", "Enter a value for b"),
							Str("default", "The b value uses component.input.a = ${component.input.a.value}"),
						),
						Block("generate_hcl",
							Labels("main.tf"),
							Block("content",
								Expr("a", `component.input.a.value`),
								Expr("b", `component.input.b.value`),
							),
						),
					),
				},
				{
					path: "/stacks/stack-1",
					add: Block("component",
						Labels("name"),
						Str("source", "/components/example.com/my-component/v1"),
						Expr("inputs", `{
							a = "AAA"
						}`),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"component_name_main.tf": stringer(`a = "AAA"
b = "The b value uses component.input.a = AAA"`),
					},
				},
			},
			wantReport: genreport.Report{
				Successes: []genreport.Result{
					{
						Dir:     project.NewPath("/stacks/stack-1"),
						Created: []string{"component_name_main.tf"},
					},
				},
			},
		},
		{
			name: "generate component with default value coming from terramate.stack.name",
			layout: []string{
				"s:stacks/stack-1",
			},
			configs: []hclconfig{
				{
					path: "/components/example.com/my-component/v1",
					add: Doc(
						Block("define",
							Labels("component", "metadata"),
							Str("class", "component-class"),
							Str("name", "my-name"),
							Str("version", "1.2.3"),
							Str("description", "My component"),
						),
						Block("define",
							Labels("component", "input", "a"),
							Str("prompt", "Enter a value"),
						),
						Block("define",
							Labels("component", "input", "b"),
							Str("prompt", "Enter a value for b"),
							Expr("default", `terramate.stack.name`),
						),
						Block("generate_hcl",
							Labels("main.tf"),
							Block("content",
								Expr("a", `component.input.a.value`),
								Expr("b", `component.input.b.value`),
							),
						),
					),
				},
				{
					path: "/stacks/stack-1",
					add: Block("component",
						Labels("name"),
						Str("source", "/components/example.com/my-component/v1"),
						Expr("inputs", `{
							a = "AAA"
						}`),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"component_name_main.tf": stringer(`a = "AAA"
b = "stack-1"`),
					},
				},
			},
			wantReport: genreport.Report{
				Successes: []genreport.Result{
					{
						Dir:     project.NewPath("/stacks/stack-1"),
						Created: []string{"component_name_main.tf"},
					},
				},
			},
		},
		{
			name: "generate component with default value coming from another default value",
			layout: []string{
				"s:stacks/stack-1",
			},
			configs: []hclconfig{
				{
					path: "/components/example.com/my-component/v1",
					add: Doc(
						Block("define",
							Labels("component", "metadata"),
							Str("class", "component-class"),
							Str("name", "my-name"),
							Str("version", "1.2.3"),
							Str("description", "My component"),
						),
						Block("define", Labels("component", "input", "a"),
							Str("prompt", "Enter a value"),
							Expr("default", `terramate.stack.name`),
						),
						Block("define", Labels("component", "input", "b"),
							Str("prompt", "Enter a value for b"),
							Expr("default", `component.input.a.value`),
						),
						Block("generate_hcl",
							Labels("main.tf"),
							Block("content",
								Expr("a", `component.input.a.value`),
								Expr("b", `component.input.b.value`),
							),
						),
					),
				},
				{
					path: "/stacks/stack-1",
					add: Block("component",
						Labels("name"),
						Str("source", "/components/example.com/my-component/v1"),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"component_name_main.tf": stringer(`a = "stack-1"
b = "stack-1"`),
					},
				},
			},
			wantReport: genreport.Report{
				Successes: []genreport.Result{
					{
						Dir:     project.NewPath("/stacks/stack-1"),
						Created: []string{"component_name_main.tf"},
					},
				},
			},
		},
		{
			name: "generate component with inputs block override component.inputs attribute",
			layout: []string{
				"s:stacks/stack-1",
			},
			configs: []hclconfig{
				{
					path: "/components/example.com/my-component/v1",
					add: Doc(
						Block("define",
							Labels("component", "metadata"),
							Str("class", "component-class"),
							Str("name", "my-name"),
							Str("version", "1.2.3"),
							Str("description", "My component"),
						),
						Block("define",
							Labels("component", "input", "test"),
							Str("prompt", "Enter a value for test"),
							Expr("type", "string"),
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
					path: "/stacks/stack-1",
					add: Doc(
						Block("globals",
							Expr("value", `{
								"test" = "value from globals block"
							}`),
						),
						Block("component",
							Labels("name"),
							Str("source", "/components/example.com/my-component/v1"),
							Expr("inputs", `global.value`),
							Block("inputs",
								Str("test", "value from inputs block"),
							),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"component_name_main.tf": stringer(`value = "value from inputs block"`),
					},
				},
			},
			wantReport: genreport.Report{
				Successes: []genreport.Result{
					{
						Dir:     project.NewPath("/stacks/stack-1"),
						Created: []string{"component_name_main.tf"},
					},
				},
			},
		},
		{
			name: "generate component with inputs block defined in separate components block",
			layout: []string{
				"s:stacks/stack-1",
			},
			configs: []hclconfig{
				{
					path: "/components/example.com/my-component/v1",
					add: Doc(
						Block("define",
							Labels("component", "metadata"),
							Str("class", "component-class"),
							Str("name", "my-name"),
							Str("version", "1.2.3"),
							Str("description", "My component"),
						),
						Block("define",
							Labels("component", "input", "test"),
							Str("prompt", "Enter a value for test"),
							Expr("type", "string"),
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
					path: "/stacks/stack-1",
					add: Doc(
						Block("globals",
							Expr("value", `{
								"test" = "value from globals block"
							}`),
						),
						Block("component",
							Labels("name"),
							Str("source", "/components/example.com/my-component/v1"),
							Expr("inputs", `global.value`),
						),
						Block("component",
							Labels("name", "inputs"),
							Str("test", "value from inputs block"),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"component_name_main.tf": stringer(`value = "value from inputs block"`),
					},
				},
			},
			wantReport: genreport.Report{
				Successes: []genreport.Result{
					{
						Dir:     project.NewPath("/stacks/stack-1"),
						Created: []string{"component_name_main.tf"},
					},
				},
			},
		},
	})
}
