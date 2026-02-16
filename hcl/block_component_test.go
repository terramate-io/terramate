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

func TestComponentBlockOKCases(t *testing.T) {
	ignoredPath := test.TempDir(t)
	ignoredRange := hhcl.Range{
		Filename: filepath.Join(ignoredPath, "defines.tm"),
	}

	newAttr := func(name string) ast.Attribute {
		return ast.NewAttribute(
			ignoredPath,
			&hhcl.Attribute{
				Name:  name,
				Range: ignoredRange,
			},
		)
	}

	newAttrPtr := func(name string) *ast.Attribute {
		attr := newAttr(name)
		return &attr
	}

	newInputsBlock := func(attrs ...ast.Attribute) *ast.MergedBlock {
		block := ast.NewMergedBlock("inputs", []string{})
		for _, attr := range attrs {
			block.Attributes[attr.Name] = attr
		}
		return block
	}

	for _, tc := range []testcase{
		{
			name: "component with source",
			input: []cfgfile{
				{filename: "component.tm", body: Block(
					"component",
					Labels("my_component"),
					Str("source", "my_source"),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Components: []*hcl.Component{
						{
							Name:   "my_component",
							Source: newAttrPtr("source"),
							Inputs: newInputsBlock(),
						},
					},
				},
			},
		},
		{
			name: "component with inputs object",
			input: []cfgfile{
				{filename: "component.tm", body: Block(
					"component",
					Labels("my_component"),
					Str("source", "my_source"),
					Expr("inputs", `{
					  name = "value"
					  name2 = "value2"
					}`),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Components: []*hcl.Component{
						{
							Name:       "my_component",
							Source:     newAttrPtr("source"),
							InputsAttr: newAttrPtr("inputs"),
							Inputs:     newInputsBlock(),
						},
					},
				},
			},
		},
		{
			name: "component with inputs block",
			input: []cfgfile{
				{filename: "component.tm", body: Block(
					"component",
					Labels("my_component"),
					Str("source", "my_source"),
					Block(
						"inputs",
						Str("name", "value"),
						Str("name2", "value2"),
					),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Components: []*hcl.Component{
						{
							Name:       "my_component",
							Source:     newAttrPtr("source"),
							InputsAttr: nil,
							Inputs: newInputsBlock(
								newAttr("name"),
								newAttr("name2"),
							),
						},
					},
				},
			},
		},
		{
			name: "component with inputs attribute and block -- no conflicts",
			input: []cfgfile{
				{filename: "component.tm", body: Block(
					"component",
					Labels("my_component"),
					Str("source", "my_source"),
					Expr("inputs", `{
					  name1 = "value1"
					  name2 = "value2"
					}`),
					Block(
						"inputs",
						Str("name3", "value3"),
						Str("name4", "value4"),
					),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Components: []*hcl.Component{
						{
							Name:       "my_component",
							Source:     newAttrPtr("source"),
							InputsAttr: newAttrPtr("inputs"),
							Inputs: newInputsBlock(
								newAttr("name3"),
								newAttr("name4"),
							),
						},
					},
				},
			},
		},
		{
			name: "component with same names in inputs attrs and blocks are allowed",
			input: []cfgfile{
				{filename: "component.tm", body: Block(
					"component",
					Labels("my_component"),
					Str("source", "my_source"),
					Expr("inputs", `{
					  name1 = "value1"
					  name2 = "value2"
					}`),
					Block(
						"inputs",
						Str("name1", "value1"),
						Str("name2", "value2"),
					),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Components: []*hcl.Component{
						{
							Name:       "my_component",
							Source:     newAttrPtr("source"),
							InputsAttr: newAttrPtr("inputs"),
							Inputs: newInputsBlock(
								newAttr("name1"),
								newAttr("name2"),
							),
						},
					},
				},
			},
		},
		{
			name: "component with same names in inputs attrs and blocks are allowed -- separate component blocks",
			input: []cfgfile{
				{filename: "component.tm", body: Doc(
					Block(
						"component",
						Labels("my_component"),
						Str("source", "my_source"),
						Expr("inputs", `{
					  name1 = "value1"
					  name2 = "value2"
					}`),
					),
					Block(
						"component",
						Labels("my_component", "inputs"),
						Str("name1", "value1"),
						Str("name2", "value2"),
					),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Components: []*hcl.Component{
						{
							Name:       "my_component",
							Source:     newAttrPtr("source"),
							InputsAttr: newAttrPtr("inputs"),
							Inputs: newInputsBlock(
								newAttr("name1"),
								newAttr("name2"),
							),
						},
					},
				},
			},
		},
	} {
		testParser(t, tc)
	}
}

func TestComponentBlockFailCases(t *testing.T) {
	for _, tc := range []testcase{
		{
			name: "component with no label",
			input: []cfgfile{
				{filename: "component.tm", body: Block(
					"component",
					Str("source", "my_source"),
				).String()},
			},
			want: want{
				errs: []error{errors.E(hcl.ErrTerramateSchema)},
			},
		},
		{
			name: "component source attribute redeclared",
			input: []cfgfile{
				{filename: "component1.tm", body: Block(
					"component",
					Labels("my_component"),
					Str("source", "my_source"),
				).String()},
				{filename: "component2.tm", body: Block(
					"component",
					Labels("my_component"),
					Str("source", "my_source"),
				).String()},
			},
			want: want{
				errs: []error{errors.E(hcl.ErrTerramateSchema)},
			},
		},
		{
			name: "component with no source",
			input: []cfgfile{
				{filename: "component.tm", body: Block(
					"component",
					Labels("my_component"),
				).String()},
			},
			want: want{
				errs: []error{errors.E(hcl.ErrComponentMissingSourceAttribute)},
			},
		},
		{
			name: "component with invalid attribute",
			input: []cfgfile{
				{filename: "component.tm", body: Block(
					"component",
					Labels("my_component"),
					Str("invalid", "invalid"),
				).String()},
			},
			want: want{
				errs: []error{errors.E(hcl.ErrComponentMissingAttribute)},
			},
		},
		{
			name: "component with inputs block with conflicts",
			input: []cfgfile{
				{filename: "component.tm", body: Block(
					"component",
					Labels("my_component"),
					Str("source", "my_source"),
					Block(
						"inputs",
						Str("name", "value"),
					),
					Block(
						"inputs",
						Str("name", "value"),
					),
				).String()},
			},
			want: want{
				errs: []error{errors.E(hcl.ErrTerramateSchema)},
			},
		},
		{
			name: "component with inputs block with conflicts from different blocks",
			input: []cfgfile{
				{filename: "component.tm", body: Doc(
					Block(
						"component",
						Labels("my_component"),
						Str("source", "my_source"),
						Block(
							"inputs",
							Str("name", "value"),
						),
					),
					Block("component",
						Labels("my_component"),
						Block(
							"inputs",
							Str("name", "value"),
						),
					),
				).String()},
			},
			want: want{
				errs: []error{errors.E(hcl.ErrTerramateSchema)},
			},
		},
		{
			name: "component block conflicting with component.name.inputs attributes",
			input: []cfgfile{
				{filename: "component.tm", body: Doc(
					Block(
						"component",
						Labels("my_component"),
						Str("source", "my_source"),
						Block(
							"inputs",
							Str("name", "value"),
						),
					),
					Block("component",
						Labels("my_component", "inputs"),
						Str("name", "value"),
					),
				).String()},
			},
			want: want{
				errs: []error{errors.E(hcl.ErrComponentInputAlreadyDeclared)},
			},
		},
		{
			name: "component block conflicting with component.name.inputs attributes -- reversed order",
			input: []cfgfile{
				{filename: "component.tm", body: Doc(
					Block("component",
						Labels("my_component", "inputs"),
						Str("name", "value"),
					),
					Block(
						"component",
						Labels("my_component"),
						Str("source", "my_source"),
						Block(
							"inputs",
							Str("name", "value"),
						),
					),
				).String()},
			},
			want: want{
				errs: []error{errors.E(hcl.ErrComponentInputAlreadyDeclared)},
			},
		},
		{
			name:      "component with inputs but missing source",
			nonStrict: true, // In strict mode the merged labels block is silently ignored.
			input: []cfgfile{
				{filename: "component.tm", body: Doc(
					Block(
						"component",
						Labels("my_component", "inputs"),
						Str("name", "value"),
					),
				).String()},
			},
			want: want{
				errs: []error{errors.E(hcl.ErrTerramateSchema)},
			},
		},
	} {
		testParser(t, tc)
	}
}
