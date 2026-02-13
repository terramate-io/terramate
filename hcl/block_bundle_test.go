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
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/test"

	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
)

func TestBundleInstantiation(t *testing.T) {
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

	newEnvironment := func(name string, attrs ...ast.Attribute) *hcl.BundleEnvValues {
		envAttr := ast.NewAttribute(
			ignoredPath,
			&hhcl.Attribute{
				Name:  name,
				Range: ignoredRange,
			},
		)
		return &hcl.BundleEnvValues{
			EnvID:  &envAttr,
			Inputs: newInputsBlock(attrs...),
		}
	}

	newEnvironmentWithSource := func(name string, source *ast.Attribute, attrs ...ast.Attribute) *hcl.BundleEnvValues {
		env := newEnvironment(name, attrs...)
		env.Source = source
		return env
	}

	for _, tc := range []testcase{
		{
			name: "basic bundle",
			input: []cfgfile{
				{filename: "bundle.tm", body: Block("bundle",
					Labels("my_bundle"),
					Str("source", "my_source"),
					Str("uuid", "19bdb9a2-46d8-4db7-8db5-4735b0582700"),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Bundles: []*hcl.BundleTemplate{
						{
							Name:    "my_bundle",
							Workdir: project.NewPath("/"),
							Source:  newAttrPtr("source"),
							UUID:    newAttrPtr("uuid"),
							Inputs:  newInputsBlock(),
						},
					},
				},
			},
		},
		{
			name: "bundle with inputs",
			input: []cfgfile{
				{filename: "bundle.tm", body: Block("bundle",
					Labels("my_bundle"),
					Str("source", "my_source"),
					Block("inputs",
						Str("name", "my_input"),
					),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Bundles: []*hcl.BundleTemplate{
						{
							Name:    "my_bundle",
							Workdir: project.NewPath("/"),
							Source:  newAttrPtr("source"),
							Inputs: newInputsBlock(
								newAttr("name"),
							),
						},
					},
				},
			},
		},
		{
			name: "bundle with multiple inputs",
			input: []cfgfile{
				{filename: "bundle.tm", body: Block("bundle",
					Labels("my_bundle"),
					Str("source", "my_source"),
					Block("inputs",
						Str("name1", "my_input1"),
						Str("name2", "my_input2"),
					),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Bundles: []*hcl.BundleTemplate{
						{
							Name:    "my_bundle",
							Workdir: project.NewPath("/"),
							Source:  newAttrPtr("source"),
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
			name: "bundle with labeled inputs",
			input: []cfgfile{
				{filename: "bundle.tm", body: Doc(
					Block("bundle",
						Labels("my_bundle", "inputs"),
						Str("name1", "my_input1"),
						Str("name2", "my_input2"),
					),
					Block("bundle",
						Labels("my_bundle"),
						Str("source", "my_source"),
					),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Bundles: []*hcl.BundleTemplate{
						{
							Name:    "my_bundle",
							Workdir: project.NewPath("/"),
							Source:  newAttrPtr("source"),
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
			name: "bundle with environment",
			input: []cfgfile{
				{filename: "bundle.tm", body: Block("bundle",
					Labels("my_bundle"),
					Str("source", "my_source"),
					Block("environment",
						Labels("prod"),
						Block("inputs",
							Str("region", "us-east-1"),
						),
					),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Bundles: []*hcl.BundleTemplate{
						{
							Name:    "my_bundle",
							Workdir: project.NewPath("/"),
							Source:  newAttrPtr("source"),
							Inputs:  newInputsBlock(),
							EnvValues: []*hcl.BundleEnvValues{
								newEnvironment("prod", newAttr("region")),
							},
						},
					},
				},
			},
		},
		{
			name: "bundle with multiple environments",
			input: []cfgfile{
				{filename: "bundle.tm", body: Block("bundle",
					Labels("my_bundle"),
					Str("source", "my_source"),
					Block("environment",
						Labels("prod"),
						Block("inputs",
							Str("region", "us-east-1"),
						),
					),
					Block("environment",
						Labels("dev"),
						Block("inputs",
							Str("region", "us-west-2"),
						),
					),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Bundles: []*hcl.BundleTemplate{
						{
							Name:    "my_bundle",
							Workdir: project.NewPath("/"),
							Source:  newAttrPtr("source"),
							Inputs:  newInputsBlock(),
							EnvValues: []*hcl.BundleEnvValues{
								newEnvironment("prod", newAttr("region")),
								newEnvironment("dev", newAttr("region")),
							},
						},
					},
				},
			},
		},
		{
			name: "bundle with environment source override",
			input: []cfgfile{
				{filename: "bundle.tm", body: Block("bundle",
					Labels("my_bundle"),
					Str("source", "my_source"),
					Block("environment",
						Labels("prod"),
						Str("source", "prod_source"),
						Block("inputs",
							Str("region", "us-east-1"),
						),
					),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Bundles: []*hcl.BundleTemplate{
						{
							Name:    "my_bundle",
							Workdir: project.NewPath("/"),
							Source:  newAttrPtr("source"),
							Inputs:  newInputsBlock(),
							EnvValues: []*hcl.BundleEnvValues{
								newEnvironmentWithSource("prod", newAttrPtr("source"), newAttr("region")),
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

func TestBundleInstantiationErrors(t *testing.T) {
	for _, tc := range []testcase{
		{
			name: "bundle with no label",
			input: []cfgfile{
				{filename: "bundle.tm", body: Block("bundle").String()},
			},
			want: want{
				errs: []error{errors.E(hcl.ErrBundleInvalidLabels)},
			},
		},
		{
			name: "bundle with invalid number of labels",
			input: []cfgfile{
				{filename: "bundle.tm", body: Block("bundle", Labels("my_bundle", "inputs", "extra")).String()},
			},
			want: want{
				errs: []error{errors.E(hcl.ErrBundleInvalidLabels)},
			},
		},
		{
			name: "bundle with invalid blocks",
			input: []cfgfile{
				{filename: "bundle.tm", body: Block(
					"bundle",
					Labels("my_bundle"),
					Str("source", "my_source"),
					Block("invalid"),
				).String()},
			},
			want: want{
				errs: []error{errors.E(hcl.ErrBundleInvalidBlock)},
			},
		},
		{
			name: "bundle with inputs sub blocks",
			input: []cfgfile{
				{filename: "bundle.tm", body: Block(
					"bundle",
					Labels("my_bundle"),
					Str("source", "my_source"),
					Block("inputs", Block("sub_block")),
				).String()},
			},
			want: want{
				errs: []error{errors.E(hcl.ErrBundleInvalidBlock)},
			},
		},
		{
			name: "bundle with no source",
			input: []cfgfile{
				{filename: "bundle.tm", body: Block(
					"bundle",
					Labels("my_bundle"),
				).String()},
			},
			want: want{
				errs: []error{errors.E(hcl.ErrBundleMissingSourceAttribute)},
			},
		},
		{
			name:      "bundle with inputs but missing source",
			nonStrict: true, // In strict mode the merged labels block is silently ignored.
			input: []cfgfile{
				{filename: "bundle.tm", body: Doc(
					Block(
						"bundle",
						Labels("my_bundle", "inputs"),
						Str("name", "value"),
					),
				).String()},
			},
			want: want{
				errs: []error{errors.E(hcl.ErrTerramateSchema)},
			},
		},
		{
			name: "bundle environment with no label",
			input: []cfgfile{
				{filename: "bundle.tm", body: Block(
					"bundle",
					Labels("my_bundle"),
					Str("source", "my_source"),
					Block("environment"),
				).String()},
			},
			want: want{
				errs: []error{errors.E(hcl.ErrTerramateSchema)},
			},
		},
		{
			name: "bundle environment with invalid sub block",
			input: []cfgfile{
				{filename: "bundle.tm", body: Block(
					"bundle",
					Labels("my_bundle"),
					Str("source", "my_source"),
					Block("environment",
						Labels("prod"),
						Block("invalid"),
					),
				).String()},
			},
			want: want{
				errs: []error{errors.E(hcl.ErrBundleInvalidBlock)},
			},
		},
	} {
		testParser(t, tc)
	}
}
