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

func TestEnvironmentBlock(t *testing.T) {
	ignoredPath := test.TempDir(t)
	ignoredRange := hhcl.Range{
		Filename: filepath.Join(ignoredPath, "config.tm"),
	}

	newAttrPtr := func(name string) *ast.Attribute {
		attr := ast.NewAttribute(
			ignoredPath,
			&hhcl.Attribute{
				Name:  name,
				Range: ignoredRange,
			},
		)
		return &attr
	}

	for _, tc := range []testcase{
		{
			name: "single environment with all attributes",
			input: []cfgfile{
				{filename: "config.tm", body: Block("environment",
					Str("id", "dev"),
					Str("name", "Development"),
					Str("description", "Development Environment"),
					Expr("promote_from", "null"),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Environments: []*hcl.Environment{
						{
							ID:          newAttrPtr("id"),
							Name:        newAttrPtr("name"),
							Description: newAttrPtr("description"),
							PromoteFrom: newAttrPtr("promote_from"),
						},
					},
				},
			},
		},
		{
			name: "multiple environments",
			input: []cfgfile{
				{filename: "config.tm", body: Doc(
					Block("environment",
						Str("id", "dev"),
						Str("name", "Development"),
						Str("description", "Development Environment"),
						Expr("promote_from", "null"),
					),
					Block("environment",
						Str("id", "stg"),
						Str("name", "Staging"),
						Str("description", "Pre-Production Environment: Staging"),
						Str("promote_from", "dev"),
					),
					Block("environment",
						Str("id", "prd"),
						Str("name", "Production"),
						Str("description", "Production Environment"),
						Str("promote_from", "stg"),
					),
					Block("environment",
						Str("id", "shr"),
						Str("name", "Shared"),
						Str("description", "A Shared Environment for global resources used by multiple environments."),
						Expr("promote_from", "null"),
					),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Environments: []*hcl.Environment{
						{
							ID:          newAttrPtr("id"),
							Name:        newAttrPtr("name"),
							Description: newAttrPtr("description"),
							PromoteFrom: newAttrPtr("promote_from"),
						},
						{
							ID:          newAttrPtr("id"),
							Name:        newAttrPtr("name"),
							Description: newAttrPtr("description"),
							PromoteFrom: newAttrPtr("promote_from"),
						},
						{
							ID:          newAttrPtr("id"),
							Name:        newAttrPtr("name"),
							Description: newAttrPtr("description"),
							PromoteFrom: newAttrPtr("promote_from"),
						},
						{
							ID:          newAttrPtr("id"),
							Name:        newAttrPtr("name"),
							Description: newAttrPtr("description"),
							PromoteFrom: newAttrPtr("promote_from"),
						},
					},
				},
			},
		},
		{
			name: "environment with minimal attributes",
			input: []cfgfile{
				{filename: "config.tm", body: Block("environment",
					Str("id", "dev"),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Environments: []*hcl.Environment{
						{
							ID: newAttrPtr("id"),
						},
					},
				},
			},
		},
	} {
		testParser(t, tc)
	}
}

func TestEnvironmentBlockErrors(t *testing.T) {
	for _, tc := range []testcase{
		{
			name: "environment with label",
			input: []cfgfile{
				{filename: "config.tm", body: Block("environment",
					Labels("dev"),
					Str("id", "dev"),
				).String()},
			},
			want: want{
				errs: []error{errors.E(hcl.ErrTerramateSchema)},
			},
		},
		{
			name: "environment with invalid attribute",
			input: []cfgfile{
				{filename: "config.tm", body: Block("environment",
					Str("id", "dev"),
					Str("invalid_attr", "value"),
				).String()},
			},
			want: want{
				errs: []error{errors.E(hcl.ErrTerramateSchema)},
			},
		},
		{
			name: "environment with nested block",
			input: []cfgfile{
				{filename: "config.tm", body: Block("environment",
					Str("id", "dev"),
					Block("nested"),
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
