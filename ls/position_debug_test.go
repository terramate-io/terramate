// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tmls

import (
	"testing"

	"github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/hcl/v2/hclsyntax"
)

func TestDebugPositions(t *testing.T) {
	content := []byte(`globals "gclz_config" "terraform" "providers" "google" "config" {
  region = "europe-west1"
}

globals "gclz_config" "terraform" "locals" {
  gclz_region = global.gclz_config.terraform.providers.google.config.region
}
`)

	file, diags := hclsyntax.ParseConfig(content, "test.tm", hcl.InitialPos)
	if diags.HasErrors() {
		t.Fatalf("Parse error: %v", diags)
	}

	syntaxBody := file.Body.(*hclsyntax.Body)

	for _, block := range syntaxBody.Blocks {
		t.Logf("Block type: %s", block.Type)
		t.Logf("Block labels: %v", block.Labels)
		t.Logf("Block TypeRange: Line %d:%d to %d:%d",
			block.TypeRange.Start.Line, block.TypeRange.Start.Column,
			block.TypeRange.End.Line, block.TypeRange.End.Column)

		for i, labelRange := range block.LabelRanges {
			t.Logf("  Label[%d] Range: Line %d:%d to %d:%d (Label: %s)",
				i,
				labelRange.Start.Line, labelRange.Start.Column,
				labelRange.End.Line, labelRange.End.Column,
				block.Labels[i])
		}

		for attrName, attr := range block.Body.Attributes {
			t.Logf("  Attribute: %s", attrName)
			t.Logf("    NameRange: Line %d:%d to %d:%d",
				attr.NameRange.Start.Line, attr.NameRange.Start.Column,
				attr.NameRange.End.Line, attr.NameRange.End.Column)
			t.Logf("    SrcRange: Line %d:%d to %d:%d",
				attr.SrcRange.Start.Line, attr.SrcRange.Start.Column,
				attr.SrcRange.End.Line, attr.SrcRange.End.Column)

			// Check if expression is a traversal
			if scopeExpr, ok := attr.Expr.(*hclsyntax.ScopeTraversalExpr); ok {
				t.Logf("    Traversal:")
				for i, part := range scopeExpr.Traversal {
					if attr, ok := part.(hcl.TraverseAttr); ok {
						t.Logf("      [%d] TraverseAttr: %s, SrcRange Line %d:%d to %d:%d",
							i, attr.Name,
							attr.SrcRange.Start.Line, attr.SrcRange.Start.Column,
							attr.SrcRange.End.Line, attr.SrcRange.End.Column)
					} else if root, ok := part.(hcl.TraverseRoot); ok {
						t.Logf("      [%d] TraverseRoot: %s, SrcRange Line %d:%d to %d:%d",
							i, root.Name,
							root.SrcRange.Start.Line, root.SrcRange.Start.Column,
							root.SrcRange.End.Line, root.SrcRange.End.Column)
					}
				}
			}
		}
		t.Logf("")
	}
}
