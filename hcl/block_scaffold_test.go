// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package hcl_test

import (
	"path/filepath"
	"testing"

	hhcl "github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/test"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
)

func TestScaffoldBlock(t *testing.T) {
	ignoredPath := test.TempDir(t)
	ignoredRange := hhcl.Range{
		Filename: filepath.Join(ignoredPath, "scaffold.tm"),
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
			name: "stack with scaffold manifests",
			input: []cfgfile{
				{filename: "scaffold.tm", body: Block(
					"scaffold",
					Expr("package_sources", `[
					  "manifest1",
					  "manifest2"
					]`),
				).String()},
			},
			want: want{
				config: hcl.Config{
					Scaffold: &hcl.Scaffold{
						PackageSources: newAttrPtr("package_sources"),
					},
				},
			},
		},
	} {
		testParser(t, tc)
	}
}
