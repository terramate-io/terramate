// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tmls

import (
	"os"

	"github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/hcl/v2/hclsyntax"
	lsp "go.lsp.dev/protocol"
)

// parseHCLFile reads and parses an HCL file, returning the syntax body
func parseHCLFile(fname string) (*hclsyntax.Body, error) {
	content, err := os.ReadFile(fname)
	if err != nil {
		return nil, err
	}

	file, diags := hclsyntax.ParseConfig(content, fname, hcl.InitialPos)
	if diags.HasErrors() {
		return nil, diags
	}

	syntaxBody, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return nil, nil
	}

	return syntaxBody, nil
}

// parseHCLContent parses HCL content from memory, returning the syntax body
func parseHCLContent(content []byte, fname string) (*hclsyntax.Body, error) {
	file, diags := hclsyntax.ParseConfig(content, fname, hcl.InitialPos)
	if diags.HasErrors() {
		return nil, diags
	}

	syntaxBody, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return nil, nil
	}

	return syntaxBody, nil
}

// hclPosToLSP converts an HCL position to LSP position
// HCL uses 1-indexed line and column, LSP uses 0-indexed
func hclPosToLSP(pos hcl.Pos) lsp.Position {
	return lsp.Position{
		Line:      uint32(pos.Line - 1),
		Character: uint32(pos.Column - 1),
	}
}

// hclRangeToLSP converts an HCL range to LSP range
// This is for ranges where both start and end need standard conversion
func hclRangeToLSP(r hcl.Range) lsp.Range {
	return lsp.Range{
		Start: hclPosToLSP(r.Start),
		End:   hclPosToLSP(r.End),
	}
}

// hclRangeToLSPSkipStartChar converts an HCL range to LSP range, skipping the first character
// Used for ranges that include a prefix character (like a dot in TraverseAttr or a quote in LabelRange)
// Start position keeps the column as-is (to skip the prefix), End position uses standard conversion
func hclRangeToLSPSkipStartChar(r hcl.Range) lsp.Range {
	return lsp.Range{
		Start: lsp.Position{
			Line:      uint32(r.Start.Line - 1),
			Character: uint32(r.Start.Column), // No -1, skip the prefix character
		},
		End: hclPosToLSP(r.End),
	}
}

// hclNameRangeToLSP converts an HCL attribute name range to LSP range
// NameRange represents just the attribute name without any prefix
func hclNameRangeToLSP(r hcl.Range) lsp.Range {
	return hclRangeToLSP(r)
}

// hclTraverseAttrToLSP converts an HCL TraverseAttr range to LSP range
// TraverseAttr.SrcRange includes the preceding dot (e.g., ".region")
func hclTraverseAttrToLSP(r hcl.Range) lsp.Range {
	return hclRangeToLSPSkipStartChar(r)
}

// hclLabelRangeToLSP converts an HCL label range to LSP range
// LabelRange includes the surrounding quotes (e.g., "google")
// Skips both opening and closing quotes to point to just the label text
func hclLabelRangeToLSP(r hcl.Range) lsp.Range {
	return lsp.Range{
		Start: lsp.Position{
			Line:      uint32(r.Start.Line - 1),
			Character: uint32(r.Start.Column), // No -1, skip opening quote
		},
		End: lsp.Position{
			Line:      uint32(r.End.Line - 1),
			Character: uint32(r.End.Column - 2), // -2 to skip closing quote (End is exclusive, so -1 for conversion, -1 for quote)
		},
	}
}
