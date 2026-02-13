// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package hcl_test

import (
	"path/filepath"
	"testing"

	hhcl "github.com/terramate-io/hcl/v2"

	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/test"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
)

func newAttribute(t *testing.T, name string) *ast.Attribute {
	ignoredPath := test.TempDir(t)
	ignoredRange := hhcl.Range{
		Filename: filepath.Join(ignoredPath, "defines.tm"),
	}
	attr := ast.NewAttribute(
		ignoredPath,
		&hhcl.Attribute{
			Name:  name,
			Range: ignoredRange,
		},
	)
	return &attr
}

func TestDefineComponentBlockOKCases(t *testing.T) {
	for _, tc := range []testcase{
		{
			name:  "no define block leaves config as nil",
			input: []cfgfile{},
			want:  want{},
		},
		{
			name: "empty define block do nothing",
			input: []cfgfile{
				{filename: "defines.tm", body: Block("define").String()},
			},
			want: want{
				config: hcl.Config{},
			},
		},
		{
			name: "multiple empty define blocks do nothing",
			input: []cfgfile{
				{filename: "defines.tm", body: Doc(
					Block("define"),
					Block("define"),
				).String()},
			},
			want: want{
				config: hcl.Config{},
			},
		},
		{
			name: "empty define with component label",
			input: []cfgfile{
				{filename: "defines.tm", body: Block("define",
					Labels("component"),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Defines: []*hcl.Define{
						{
							Component: &hcl.DefineComponent{
								Inputs: map[string]*hcl.DefineInput{},
							},
						},
					},
				},
			},
		},
		{
			name: "define block with one component with metadata and no inputs",
			input: []cfgfile{
				{filename: "defines.tm", body: Block("define",
					Block("component",
						Block("metadata",
							Str("class", "component_class"),
							Str("name", "component_name"),
							Str("version", "1.0.0"),
						),
					),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Defines: []*hcl.Define{
						{
							Component: &hcl.DefineComponent{
								Metadata: hcl.Metadata{
									Class:   newAttribute(t, "class"),
									Name:    newAttribute(t, "name"),
									Version: newAttribute(t, "version"),
								},
								Inputs: map[string]*hcl.DefineInput{},
							},
						},
					},
				},
			},
		},
		{
			name: "define block with technologies but not validate them",
			input: []cfgfile{
				{filename: "defines.tm", body: Block("define",
					Block("component",
						Block("metadata",
							Str("class", "component_class"),
							Str("name", "component_name"),
							Str("version", "1.0.0"),
							Str("description", "component_description"),
							Expr("technologies", `["terraform", "aws"]`),
						),
					),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Defines: []*hcl.Define{
						{
							Component: &hcl.DefineComponent{
								Metadata: hcl.Metadata{
									Class:        newAttribute(t, "class"),
									Name:         newAttribute(t, "name"),
									Version:      newAttribute(t, "version"),
									Description:  newAttribute(t, "description"),
									Technologies: newAttribute(t, "technologies"),
								},
								Inputs: map[string]*hcl.DefineInput{},
							},
						},
					},
				},
			},
		},
		{
			name: "define block with component and multiple inputs",
			input: []cfgfile{
				{filename: "defines.tm", body: Block("define",
					Block("component",
						Block("input",
							Labels("input1"),
							Str("prompt", "input1_prompt"),
							Str("description", "input1_description"),
						),
						Block("input",
							Labels("input2"),
							Str("prompt", "input2_prompt"),
							Str("description", "input2_description"),
							Str("default", "input2_default"),
						),
					),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Defines: []*hcl.Define{
						{
							Component: &hcl.DefineComponent{
								Inputs: map[string]*hcl.DefineInput{
									"input1": {
										Name:        "input1",
										Prompt:      newAttribute(t, "prompt"),
										Description: newAttribute(t, "description"),
									},
									"input2": {
										Name:        "input2",
										Prompt:      newAttribute(t, "prompt"),
										Description: newAttribute(t, "description"),
										Default:     newAttribute(t, "default"),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "define input with optional allowed values but not validate them",
			input: []cfgfile{
				{filename: "defines.tm", body: Block("define",
					Block("component",
						Block("input",
							Labels("input1"),
							Str("prompt", "input1_prompt"),
							Str("description", "input1_description"),
							Expr("options", `["value1", "value2"]`),
						),
						Block("input",
							Labels("input2"),
							Str("prompt", "input2_prompt"),
							Str("description", "input2_description"),
						),
					),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Defines: []*hcl.Define{
						{
							Component: &hcl.DefineComponent{
								Inputs: map[string]*hcl.DefineInput{
									"input1": {
										Name:        "input1",
										Prompt:      newAttribute(t, "prompt"),
										Description: newAttribute(t, "description"),
										Options:     newAttribute(t, "options"),
									},
									"input2": {
										Name:        "input2",
										Prompt:      newAttribute(t, "prompt"),
										Description: newAttribute(t, "description"),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "define input with optional type but not validate it",
			input: []cfgfile{
				{filename: "defines.tm", body: Block("define",
					Block("component",
						Block("input",
							Labels("input1"),
							Str("prompt", "input1_prompt"),
							Str("description", "input1_description"),
							Expr("type", `string`),
						),
						Block("input",
							Labels("input2"),
							Str("prompt", "input2_prompt"),
							Str("description", "input2_description"),
							Expr("type", `list(string)`),
						),
					),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Defines: []*hcl.Define{
						{
							Component: &hcl.DefineComponent{
								Inputs: map[string]*hcl.DefineInput{
									"input1": {
										Name:        "input1",
										Prompt:      newAttribute(t, "prompt"),
										Description: newAttribute(t, "description"),
										Type:        newAttribute(t, "type"),
									},
									"input2": {
										Name:        "input2",
										Prompt:      newAttribute(t, "prompt"),
										Description: newAttribute(t, "description"),
										Type:        newAttribute(t, "type"),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "define block with component and metadata in one file, inputs in another",
			input: []cfgfile{
				{filename: "metadata.tm", body: Block("define",
					Block("component",
						Block("metadata",
							Str("class", "component_class"),
							Str("name", "component_name"),
							Str("version", "component_version"),
							Str("description", "component_description"),
						),
					),
				).String()},
				{filename: "inputs.tm", body: Block("define",
					Block("component",
						Block("input",
							Labels("input1"),
							Str("prompt", "input1_prompt"),
							Str("description", "input1_description"),
						),
						Block("input",
							Labels("input2"),
							Str("prompt", "input2_prompt"),
							Str("description", "input2_description"),
						),
					),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Defines: []*hcl.Define{
						{
							Component: &hcl.DefineComponent{
								Metadata: hcl.Metadata{
									Class:       newAttribute(t, "class"),
									Name:        newAttribute(t, "name"),
									Version:     newAttribute(t, "version"),
									Description: newAttribute(t, "description"),
								},
								Inputs: map[string]*hcl.DefineInput{
									"input1": {
										Name:        "input1",
										Prompt:      newAttribute(t, "prompt"),
										Description: newAttribute(t, "description"),
									},
									"input2": {
										Name:        "input2",
										Prompt:      newAttribute(t, "prompt"),
										Description: newAttribute(t, "description"),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "advanced setup using multiple files",
			input: []cfgfile{
				{filename: "define-metadata-id.tm", body: Block("define",
					Labels("component", "metadata"),
					Str("class", "component_class"),
				).String()},
				{filename: "define-metadata-name.tm", body: Block("define",
					Labels("component", "metadata"),
					Str("name", "component_name"),
				).String()},
				{filename: "define-metadata-description.tm", body: Block("define",
					Labels("component", "metadata"),
					Str("description", "component_description"),
				).String()},
				{filename: "define-metadata-version.tm", body: Block("define",
					Labels("component", "metadata"),
					Str("version", "1.2.3"),
				).String()},
				{filename: "define-input1.tm", body: Block("define",
					Labels("component", "input", "input1"),
					Str("prompt", "input1_prompt"),
					Str("description", "input1_description"),
				).String()},
				{filename: "define-input2.tm", body: Block("define",
					Labels("component", "input", "input2"),
					Str("prompt", "input2_prompt"),
					Str("description", "input2_description"),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Defines: []*hcl.Define{
						{
							Component: &hcl.DefineComponent{
								Metadata: hcl.Metadata{
									Class:       newAttribute(t, "class"),
									Name:        newAttribute(t, "name"),
									Version:     newAttribute(t, "version"),
									Description: newAttribute(t, "description"),
								},
								Inputs: map[string]*hcl.DefineInput{
									"input1": {
										Name:        "input1",
										Prompt:      newAttribute(t, "prompt"),
										Description: newAttribute(t, "description"),
									},
									"input2": {
										Name:        "input2",
										Prompt:      newAttribute(t, "prompt"),
										Description: newAttribute(t, "description"),
									},
								},
							},
						},
					},
				},
			},
		},
	} {
		testParser(t, tc)
	}
}

func TestDefineBundleBlockOKCases(t *testing.T) {
	for _, tc := range []testcase{
		{
			name: "define bundle block with metadata and no inputs or exports",
			input: []cfgfile{
				{filename: "defines.tm", body: Block("define",
					Labels("bundle"),
					Block("metadata",
						Str("class", "bundle_class"),
						Str("name", "component_name"),
						Str("version", "component_version"),
					),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Defines: []*hcl.Define{
						{
							Bundle: &hcl.DefineBundle{
								Stacks: map[string]*hcl.DefineStack{},
								Metadata: hcl.Metadata{
									Class:   newAttribute(t, "class"),
									Name:    newAttribute(t, "name"),
									Version: newAttribute(t, "version"),
								},
								Inputs:  map[string]*hcl.DefineInput{},
								Exports: map[string]*hcl.DefineExport{},
							},
						},
					},
				},
			},
		},
		{
			name: "define bundle and component as child of define unlabeled block",
			input: []cfgfile{
				{filename: "defines.tm", body: Block("define",
					Block("bundle",
						Block("metadata",
							Str("class", "bundle_class"),
							Str("name", "bundle_name"),
							Str("version", "1.2.3"),
						),
					),
					Block("component",
						Block("metadata",
							Str("class", "component_class"),
							Str("name", "component_name"),
							Str("version", "1.2.3"),
						),
					),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Defines: []*hcl.Define{
						{
							Bundle: &hcl.DefineBundle{
								Stacks: map[string]*hcl.DefineStack{},
								Metadata: hcl.Metadata{
									Class:   newAttribute(t, "class"),
									Name:    newAttribute(t, "name"),
									Version: newAttribute(t, "version"),
								},
								Inputs:  map[string]*hcl.DefineInput{},
								Exports: map[string]*hcl.DefineExport{},
							},
							Component: &hcl.DefineComponent{
								Metadata: hcl.Metadata{
									Class:   newAttribute(t, "class"),
									Name:    newAttribute(t, "name"),
									Version: newAttribute(t, "version"),
								},
								Inputs: map[string]*hcl.DefineInput{},
							},
						},
					},
				},
			},
		},
		{
			name: "define bundle and component metadata using define labels",
			input: []cfgfile{
				{filename: "defines.tm", body: Doc(
					Block("define",
						Labels("bundle", "metadata"),
						Str("class", "bundle_class"),
						Str("name", "bundle_name"),
						Str("version", "1.2.3"),
					),
					Block("define",
						Labels("component", "metadata"),
						Str("class", "component_class"),
						Str("name", "component_name"),
						Str("version", "1.2.3"),
					),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Defines: []*hcl.Define{
						{
							Bundle: &hcl.DefineBundle{
								Stacks: map[string]*hcl.DefineStack{},
								Metadata: hcl.Metadata{
									Class:   newAttribute(t, "class"),
									Name:    newAttribute(t, "name"),
									Version: newAttribute(t, "version"),
								},
								Inputs:  map[string]*hcl.DefineInput{},
								Exports: map[string]*hcl.DefineExport{},
							},
							Component: &hcl.DefineComponent{
								Metadata: hcl.Metadata{
									Class:   newAttribute(t, "class"),
									Name:    newAttribute(t, "name"),
									Version: newAttribute(t, "version"),
								},
								Inputs: map[string]*hcl.DefineInput{},
							},
						},
					},
				},
			},
		},
		{
			name: "define bundle and component metadata using define labels - reverse order",
			input: []cfgfile{
				{filename: "defines.tm", body: Doc(
					Block("define",
						Labels("component", "metadata"),
						Str("class", "component_class"),
						Str("name", "bundle_name"),
						Str("version", "1.2.3"),
					),
					Block("define",
						Labels("bundle", "metadata"),
						Str("class", "bundle_class"),
						Str("name", "component_name"),
						Str("version", "1.2.3"),
					),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Defines: []*hcl.Define{
						{
							Bundle: &hcl.DefineBundle{
								Stacks: map[string]*hcl.DefineStack{},
								Metadata: hcl.Metadata{
									Class:   newAttribute(t, "class"),
									Name:    newAttribute(t, "name"),
									Version: newAttribute(t, "version"),
								},
								Inputs:  map[string]*hcl.DefineInput{},
								Exports: map[string]*hcl.DefineExport{},
							},
							Component: &hcl.DefineComponent{
								Metadata: hcl.Metadata{
									Class:   newAttribute(t, "class"),
									Name:    newAttribute(t, "name"),
									Version: newAttribute(t, "version"),
								},
								Inputs: map[string]*hcl.DefineInput{},
							},
						},
					},
				},
			},
		},
		{
			name: "define bundle with inputs and exports inside unlabeled define block",
			input: []cfgfile{
				{filename: "defines.tm", body: Block("define",
					Block("bundle",
						Block("input",
							Labels("input1_name"),
							Str("prompt", "input1_prompt"),
							Str("description", "input1_description"),
							Expr("type", `string`),
						),
						Block("input",
							Labels("input2_name"),
							Str("prompt", "input2_prompt"),
							Str("description", "input2_description"),
							Expr("type", `string`),
						),
						Block("export",
							Labels("export1_name"),
							Str("description", "export1_description"),
							Expr("value", `bundle.inputs.input1_name`),
						),
						Block("export",
							Labels("export2_name"),
							Str("description", "export2_description"),
							Expr("value", `bundle.inputs.input2_name`),
						),
					),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Defines: []*hcl.Define{
						{
							Bundle: &hcl.DefineBundle{
								Stacks: map[string]*hcl.DefineStack{},
								Inputs: map[string]*hcl.DefineInput{
									"input1_name": {
										Name:        "input1_name",
										Prompt:      newAttribute(t, "prompt"),
										Description: newAttribute(t, "description"),
										Type:        newAttribute(t, "type"),
									},
									"input2_name": {
										Name:        "input2_name",
										Prompt:      newAttribute(t, "prompt"),
										Description: newAttribute(t, "description"),
										Type:        newAttribute(t, "type"),
									},
								},
								Exports: map[string]*hcl.DefineExport{
									"export1_name": {
										Name:        "export1_name",
										Description: newAttribute(t, "description"),
										Value:       newAttribute(t, "value"),
									},
									"export2_name": {
										Name:        "export2_name",
										Description: newAttribute(t, "description"),
										Value:       newAttribute(t, "value"),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "define bundle with inputs inside labeled defined block",
			input: []cfgfile{
				{filename: "defines.tm", body: Block("define",
					Labels("bundle"),
					Block("input",
						Labels("input1_name"),
						Str("prompt", "input1_prompt"),
						Str("description", "input1_description"),
						Expr("type", `string`),
					),
					Block("input",
						Labels("input2_name"),
						Str("prompt", "input2_prompt"),
						Str("description", "input2_description"),
						Expr("type", `string`),
					),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Defines: []*hcl.Define{
						{
							Bundle: &hcl.DefineBundle{
								Stacks: map[string]*hcl.DefineStack{},
								Inputs: map[string]*hcl.DefineInput{
									"input1_name": {
										Name:        "input1_name",
										Prompt:      newAttribute(t, "prompt"),
										Description: newAttribute(t, "description"),
										Type:        newAttribute(t, "type"),
									},
									"input2_name": {
										Name:        "input2_name",
										Prompt:      newAttribute(t, "prompt"),
										Description: newAttribute(t, "description"),
										Type:        newAttribute(t, "type"),
									},
								},
								Exports: map[string]*hcl.DefineExport{},
							},
						},
					},
				},
			},
		},
		{
			name: "define inputs for bundle and component in define labeled define block",
			input: []cfgfile{
				{filename: "defines.tm", body: Doc(
					Block("define",
						Labels("bundle", "input", "input1_name"),
						Str("prompt", "input1_prompt"),
						Str("description", "input1_description"),
						Expr("type", `string`),
					),
					Block("define",
						Labels("bundle", "input", "input2_name"),
						Str("prompt", "input2_prompt"),
						Str("description", "input2_description"),
						Expr("type", `string`),
					),
					Block("define",
						Labels("component", "input", "input1_name"),
						Str("prompt", "input1_prompt"),
						Str("description", "input1_description"),
						Expr("type", `string`),
					),
					Block("define",
						Labels("component", "input", "input2_name"),
						Str("prompt", "input2_prompt"),
						Str("description", "input2_description"),
						Expr("type", `string`),
					),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Defines: []*hcl.Define{
						{
							Bundle: &hcl.DefineBundle{
								Stacks: map[string]*hcl.DefineStack{},
								Inputs: map[string]*hcl.DefineInput{
									"input1_name": {
										Name:        "input1_name",
										Prompt:      newAttribute(t, "prompt"),
										Description: newAttribute(t, "description"),
										Type:        newAttribute(t, "type"),
									},
									"input2_name": {
										Name:        "input2_name",
										Prompt:      newAttribute(t, "prompt"),
										Description: newAttribute(t, "description"),
										Type:        newAttribute(t, "type"),
									},
								},
								Exports: map[string]*hcl.DefineExport{},
							},
							Component: &hcl.DefineComponent{
								Inputs: map[string]*hcl.DefineInput{
									"input1_name": {
										Name:        "input1_name",
										Prompt:      newAttribute(t, "prompt"),
										Description: newAttribute(t, "description"),
										Type:        newAttribute(t, "type"),
									},
									"input2_name": {
										Name:        "input2_name",
										Prompt:      newAttribute(t, "prompt"),
										Description: newAttribute(t, "description"),
										Type:        newAttribute(t, "type"),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "define bundle with stack in unlabeled define block",
			input: []cfgfile{
				{filename: "defines.tm", body: Block("define",
					Block("bundle",
						Block("stack",
							Labels("some_name"),
							Block("metadata",
								Str("name", "stack_name"),
								Str("description", "stack_description"),
								Str("path", "stack_path"),
							),
						),
					),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Defines: []*hcl.Define{
						{
							Bundle: &hcl.DefineBundle{
								Stacks: map[string]*hcl.DefineStack{
									"some_name": {
										Metadata: hcl.StackMetadata{
											Name:        newAttribute(t, "name"),
											Description: newAttribute(t, "description"),
											Path:        newAttribute(t, "path"),
										},
									},
								},
								Inputs:  map[string]*hcl.DefineInput{},
								Exports: map[string]*hcl.DefineExport{},
							},
						},
					},
				},
			},
		},
		{
			name: "define bundle with stack in labeled define block",
			input: []cfgfile{
				{filename: "defines.tm", body: Block("define",
					Labels("bundle"),
					Block("stack",
						Labels("some_name"),
						Block("metadata",
							Str("name", "stack_name"),
							Str("description", "stack_description"),
							Str("path", "stack_path"),
						),
					),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Defines: []*hcl.Define{
						{
							Bundle: &hcl.DefineBundle{
								Stacks: map[string]*hcl.DefineStack{
									"some_name": {
										Metadata: hcl.StackMetadata{
											Name:        newAttribute(t, "name"),
											Description: newAttribute(t, "description"),
											Path:        newAttribute(t, "path"),
										},
									},
								},
								Inputs:  map[string]*hcl.DefineInput{},
								Exports: map[string]*hcl.DefineExport{},
							},
						},
					},
				},
			},
		},
		{
			name: "define bundle stack with label",
			input: []cfgfile{
				{filename: "defines.tm", body: Block("define",
					Labels("bundle", "stack", "name"),
					Block("metadata",
						Str("name", "stack_name"),
						Str("description", "stack_description"),
						Str("path", "stack_path"),
					),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Defines: []*hcl.Define{
						{
							Bundle: &hcl.DefineBundle{
								Inputs:  map[string]*hcl.DefineInput{},
								Exports: map[string]*hcl.DefineExport{},
								Stacks: map[string]*hcl.DefineStack{
									"name": {
										Metadata: hcl.StackMetadata{
											Name:        newAttribute(t, "name"),
											Description: newAttribute(t, "description"),
											Path:        newAttribute(t, "path"),
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "define.bundle.stack.<label>.metadata attributes",
			input: []cfgfile{
				{filename: "defines.tm", body: Block("define",
					Labels("bundle", "stack", "name", "metadata"),
					Str("name", "stack_name"),
					Str("description", "stack_description"),
					Str("path", "stack_path"),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Defines: []*hcl.Define{
						{
							Bundle: &hcl.DefineBundle{
								Stacks: map[string]*hcl.DefineStack{
									"name": {
										Metadata: hcl.StackMetadata{
											Name:        newAttribute(t, "name"),
											Description: newAttribute(t, "description"),
											Path:        newAttribute(t, "path"),
										},
									},
								},
								Inputs:  map[string]*hcl.DefineInput{},
								Exports: map[string]*hcl.DefineExport{},
							},
						},
					},
				},
			},
		},
	} {
		testParser(t, tc)
	}
}

func TestDefineSchemaBlockOKCases(t *testing.T) {
	for _, tc := range []testcase{
		{
			name: "define schema using nested syntax: define { schema name { } }",
			input: []cfgfile{
				{filename: "defines.tm", body: Block("define",
					Block("schema",
						Labels("my_schema"),
						Str("description", "my schema description"),
						Expr("type", `string`),
					),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Defines: []*hcl.Define{
						{
							Schemas: []*hcl.DefineSchema{
								{
									Name:        "my_schema",
									Description: newAttribute(t, "description"),
									Type:        newAttribute(t, "type"),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "define schema using labeled syntax: define schema name { }",
			input: []cfgfile{
				{filename: "defines.tm", body: Block("define",
					Labels("schema", "my_schema"),
					Str("description", "my schema description"),
					Expr("type", `string`),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Defines: []*hcl.Define{
						{
							Schemas: []*hcl.DefineSchema{
								{
									Name:        "my_schema",
									Description: newAttribute(t, "description"),
									Type:        newAttribute(t, "type"),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "define schema with object attribute using nested syntax",
			input: []cfgfile{
				{filename: "defines.tm", body: Block("define",
					Block("schema",
						Labels("my_schema"),
						Str("description", "my schema description"),
						Block("attribute",
							Labels("attr1"),
							Str("description", "attribute 1 description"),
							Expr("type", `string`),
							Expr("required", `true`),
						),
					),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Defines: []*hcl.Define{
						{
							Schemas: []*hcl.DefineSchema{
								{
									Name:        "my_schema",
									Description: newAttribute(t, "description"),
									ObjectAttributes: []*hcl.DefineObjectAttribute{
										{
											Name:        "attr1",
											Description: newAttribute(t, "description"),
											Type:        newAttribute(t, "type"),
											Required:    newAttribute(t, "required"),
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "define schema with object attributes using labeled syntax",
			input: []cfgfile{
				{filename: "defines.tm", body: Block("define",
					Labels("schema", "my_schema"),
					Str("description", "my schema description"),
					Block("attribute",
						Labels("attr1"),
						Str("description", "attribute 1 description"),
						Expr("type", `string`),
					),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Defines: []*hcl.Define{
						{
							Schemas: []*hcl.DefineSchema{
								{
									Name:        "my_schema",
									Description: newAttribute(t, "description"),
									ObjectAttributes: []*hcl.DefineObjectAttribute{
										{
											Name:        "attr1",
											Description: newAttribute(t, "description"),
											Type:        newAttribute(t, "type"),
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "define multiple schemas using mixed syntax",
			input: []cfgfile{
				{filename: "defines.tm", body: Doc(
					Block("define",
						Block("schema",
							Labels("schema_nested"),
							Str("description", "nested schema"),
						),
					),
					Block("define",
						Labels("schema", "schema_labeled"),
						Str("description", "labeled schema"),
					),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Defines: []*hcl.Define{
						{
							Schemas: []*hcl.DefineSchema{
								{
									Name:        "schema_labeled",
									Description: newAttribute(t, "description"),
								},
								{
									Name:        "schema_nested",
									Description: newAttribute(t, "description"),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "define schema alongside bundle and component",
			input: []cfgfile{
				{filename: "defines.tm", body: Block("define",
					Block("bundle",
						Block("metadata",
							Str("class", "bundle_class"),
							Str("name", "bundle_name"),
							Str("version", "1.0.0"),
						),
					),
					Block("schema",
						Labels("my_schema"),
						Str("description", "my schema"),
					),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Defines: []*hcl.Define{
						{
							Bundle: &hcl.DefineBundle{
								Stacks: map[string]*hcl.DefineStack{},
								Metadata: hcl.Metadata{
									Class:   newAttribute(t, "class"),
									Name:    newAttribute(t, "name"),
									Version: newAttribute(t, "version"),
								},
								Inputs:  map[string]*hcl.DefineInput{},
								Exports: map[string]*hcl.DefineExport{},
							},
							Schemas: []*hcl.DefineSchema{
								{
									Name:        "my_schema",
									Description: newAttribute(t, "description"),
								},
							},
						},
					},
				},
			},
		},
	} {
		testParser(t, tc)
	}
}

func TestDefineBlockFailCases(t *testing.T) {
	for _, tc := range []testcase{
		{
			name: "define component metadata without version",
			input: []cfgfile{
				{filename: "defines.tm", body: Doc(
					Block("define",
						Block("component",
							Block("metadata",
								Str("class", "component_class"),
								Str("name", "component_name"),
							),
						),
					),
				).String()},
			},
			want: want{
				errs: []error{errors.E(hcl.ErrMissingAttribute("metadata", "version"))},
			},
		},
		{
			name: "define component metadata without class",
			input: []cfgfile{
				{filename: "defines.tm", body: Doc(
					Block("define",
						Block("component",
							Block("metadata",
								Str("version", "component_version"),
								Str("name", "component_name"),
							),
						),
					),
				).String()},
			},
			want: want{
				errs: []error{errors.E(hcl.ErrMissingAttribute("metadata", "class"))},
			},
		},
		{
			name: "define bundle metadata without name",
			input: []cfgfile{
				{filename: "defines.tm", body: Doc(
					Block("define",
						Block("bundle",
							Block("metadata",
								Str("version", "bundle_version"),
								Str("class", "bundle_class"),
							),
						),
					),
				).String()},
			},
			want: want{
				errs: []error{errors.E(hcl.ErrMissingAttribute("metadata", "name"))},
			},
		},
		// merging issues
		{
			name: "define blocks with metadata attributes redeclared",
			input: []cfgfile{
				{filename: "defines.tm", body: Doc(
					Block("define",
						Block("component",
							Block("metadata",
								Str("class", "component1_class"),
							),
						),
					),
					Block("define",
						Block("component",
							Block("metadata",
								Str("class", "component2_class"),
							),
						),
					),
				).String()},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema),
					errors.E(hcl.ErrTerramateSchema),
				},
			},
		},
		{
			name: "define blocks with metadata attributes redeclared using labels",
			input: []cfgfile{
				{filename: "defines.tm", body: Doc(
					Block("define",
						Labels("component", "metadata"),
						Str("class", "abc"),
					),
					Block("define",
						Labels("component", "metadata"),
						Str("class", "xyz"),
					),
				).String()},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema),
					errors.E(hcl.ErrTerramateSchema),
				},
			},
		},
		{
			name: "define blocks with metadata attributes redeclared using mixed approaches",
			input: []cfgfile{
				{filename: "defines.tm", body: Doc(
					Block("define",
						Labels("component", "metadata"),
						Str("class", "component1_class"),
					),
					Block("define",
						Block("component",
							Block("metadata",
								Str("class", "component2_class"),
							),
						),
					),
				).String()},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema),
					errors.E(hcl.ErrTerramateSchema),
				},
			},
		},
		{
			name: "define blocks with metadata attributes redeclared (in separate files)",
			input: []cfgfile{
				{filename: "define1.tm", body: Block("define",
					Block("component",
						Block("metadata",
							Str("class", "component1_class"),
						),
					),
				).String()},
				{filename: "define2.tm", body: Block("define",
					Block("component",
						Block("metadata",
							Str("class", "component2_class"),
						),
					),
				).String()},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema),
					errors.E(hcl.ErrTerramateSchema),
				},
			},
		},
		{
			name: "define blocks with metadata attributes redeclared using labels (in separate files)",
			input: []cfgfile{
				{filename: "define1.tm", body: Block("define",
					Labels("component", "metadata"),
					Str("class", "abc"),
				).String()},
				{filename: "define2.tm", body: Block("define",
					Labels("component", "metadata"),
					Str("class", "xyz"),
				).String()},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema),
					errors.E(hcl.ErrTerramateSchema),
				},
			},
		},

		// unrecognized blocks
		{
			name: "sanity check -- top-level unrecognized block",
			input: []cfgfile{
				{filename: "sanity.tm", body: Block("this_should_fail").String()},
			},
			want: want{
				errs: []error{errors.E(hcl.ErrTerramateSchema)},
			},
		},
		{
			name: "define block with unrecognized sub block",
			input: []cfgfile{
				{filename: "defines.tm", body: Block("define",
					Block("this_should_fail"),
				).String()},
			},
			want: want{
				errs: []error{errors.E(hcl.ErrUnrecognizedDefineSubBlock)},
			},
		},
		{
			name: "component block with unrecognized sub block",
			input: []cfgfile{
				{filename: "defines.tm", body: Block("define",
					Block("component",
						Block("this_should_fail"),
					),
				).String()},
			},
			want: want{
				errs: []error{errors.E(hcl.ErrUnrecognizedDefineSubBlock)},
			},
		},
		{
			name: "input block with unrecognized sub block",
			input: []cfgfile{
				{filename: "defines.tm", body: Block("define",
					Block("component",
						Block("input",
							Labels("name"),
							Block("this_should_fail"),
						),
					),
				).String()},
			},
			want: want{
				errs: []error{errors.E(hcl.ErrUnrecognizedDefineSubBlock)},
			},
		},
		{
			name: "metadata block with unrecognized sub block",
			input: []cfgfile{
				{filename: "defines.tm", body: Block("define",
					Block("component",
						Block("metadata",
							Block("this_should_fail"),
						),
					),
				).String()},
			},
			want: want{
				errs: []error{errors.E(hcl.ErrUnrecognizedMetadataBlock)},
			},
		},
		{
			name: "define block with unrecognized attribute",
			input: []cfgfile{
				{filename: "defines.tm", body: Block("define",
					Str("this_should_fail", "value"),
				).String()},
			},
			want: want{
				errs: []error{errors.E(hcl.ErrUnrecognizedDefineAttribute)},
			},
		},
		{
			name: "metadata block with unrecognized attribute",
			input: []cfgfile{
				{filename: "defines.tm", body: Block("define",
					Block("component",
						Block("metadata",
							Str("this_should_fail", "value"),
						),
					),
				).String()},
			},
			want: want{
				errs: []error{errors.E(hcl.ErrUnrecognizedComponentMetadataAttribute)},
			},
		},
		{
			name: "input block with unrecognized attribute",
			input: []cfgfile{
				{filename: "defines.tm", body: Block("define",
					Block("component",
						Block("input",
							Labels("name"),
							Str("this_should_fail", "value"),
						),
					),
				).String()},
			},
			want: want{
				errs: []error{errors.E(hcl.ErrUnrecognizedInputAttribute)},
			},
		},
		{
			name: "define bundle stack with label and invalid attributes -- they must be defined in metadata",
			input: []cfgfile{
				{filename: "defines.tm", body: Block("define",
					Labels("bundle", "stack", "name"),
					Str("name", "stack_name"),
					Str("description", "stack_description"),
					Str("path", "stack_path"),
				).String()},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrUnrecognizedStackAttribute),
					errors.E(hcl.ErrUnrecognizedStackAttribute),
					errors.E(hcl.ErrUnrecognizedStackAttribute),
				},
			},
		},
	} {
		testParser(t, tc)
	}
}
