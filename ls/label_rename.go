// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tmls

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/hcl/v2/hclsyntax"
	lsp "go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// renameLabelComponent renames a label in a labeled globals block
// For example, renaming "providers" in globals "a" "b" "providers" "c" { ... }
func (s *Server) renameLabelComponent(ctx context.Context, fname string, info *symbolInfo, newName string) (*lsp.WorkspaceEdit, error) {
	if !info.isLabelComponent {
		return nil, nil
	}

	if info.componentIndex < 0 || info.componentIndex >= len(info.pathComponents) {
		return nil, nil
	}

	// The component being renamed
	oldName := info.pathComponents[info.componentIndex]

	// Build the label path up to and including the component being renamed
	// For renaming "providers" in ["gclz_config", "terraform", "providers", "google"]
	// We get: ["gclz_config", "terraform", "providers"]
	labelPath := info.pathComponents[:info.componentIndex+1]

	changes := make(map[lsp.DocumentURI][]lsp.TextEdit)

	// Step 1: Find and update the labeled block definitions
	blockEdits := s.findAndRenameLabeledBlocks(fname, labelPath, oldName, newName)
	for uri, edits := range blockEdits {
		changes[uri] = append(changes[uri], edits...)
	}

	// Step 2: Find and update all references
	refEdits := s.findAndRenamePathReferences(ctx, fname, info.componentIndex, oldName, newName, info.pathComponents)
	for uri, edits := range refEdits {
		changes[uri] = append(changes[uri], edits...)
	}

	if len(changes) == 0 {
		return nil, nil
	}

	return &lsp.WorkspaceEdit{
		Changes: changes,
	}, nil
}

// findAndRenameLabeledBlocks finds all labeled blocks with the given label path
// and creates edits to rename the specific label
func (s *Server) findAndRenameLabeledBlocks(startFile string, labelPath []string, oldName, newName string) map[lsp.DocumentURI][]lsp.TextEdit {
	edits := make(map[lsp.DocumentURI][]lsp.TextEdit)
	visited := make(map[string]bool)

	// Search current directory and parents (including imports)
	dir := filepath.Dir(startFile)
	for {
		// Search this directory and its imports
		s.findLabeledBlocksInDir(dir, labelPath, oldName, newName, edits, visited)

		// Move to parent
		parent := filepath.Dir(dir)
		if parent == dir || !strings.HasPrefix(parent, s.workspace) {
			break
		}
		dir = parent
	}

	return edits
}

// findLabeledBlocksInDir finds labeled blocks in a directory and its imports.
func (s *Server) findLabeledBlocksInDir(dir string, labelPath []string, oldName, newName string,
	edits map[lsp.DocumentURI][]lsp.TextEdit, visited map[string]bool) {

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		if !isTerramateFile(filename) {
			continue
		}

		fullPath := filepath.Join(dir, filename)
		if visited[fullPath] {
			continue
		}
		visited[fullPath] = true

		s.renameLabelInFile(fullPath, labelPath, oldName, newName, edits)
	}
	imports := s.collectImportsFromDir(dir, visited)
	for _, importFile := range imports {
		if visited[importFile] {
			continue
		}
		visited[importFile] = true
		s.renameLabelInFile(importFile, labelPath, oldName, newName, edits)
	}
}

// renameLabelInFile finds labeled blocks in a file and creates rename edits
func (s *Server) renameLabelInFile(fname string, labelPath []string, _, newName string,
	edits map[lsp.DocumentURI][]lsp.TextEdit) {

	content, err := os.ReadFile(fname)
	if err != nil {
		return
	}

	file, diags := hclsyntax.ParseConfig(content, fname, hcl.InitialPos)
	if diags.HasErrors() {
		return
	}

	syntaxBody, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return
	}

	_ = hclsyntax.VisitAll(syntaxBody, func(node hclsyntax.Node) hcl.Diagnostics {
		if block, ok := node.(*hclsyntax.Block); ok {
			if block.Type == "globals" && len(block.Labels) > 0 {
				// Check if this block's labels match our label path
				// For labelPath ["a", "b", "c"], we match blocks starting with these labels
				if len(block.Labels) >= len(labelPath) {
					matches := true
					for i, label := range labelPath {
						if block.Labels[i] != label {
							matches = false
							break
						}
					}

					if matches {
						// This block has the label we want to rename
						// The label to rename is at index (len(labelPath) - 1)
						labelIdx := len(labelPath) - 1

						// Create edit for this label
						labelRange := block.LabelRanges[labelIdx]
						edit := lsp.TextEdit{
							Range: lsp.Range{
								Start: lsp.Position{
									Line:      uint32(labelRange.Start.Line - 1),
									Character: uint32(labelRange.Start.Column), // Skip opening quote
								},
								End: lsp.Position{
									Line:      uint32(labelRange.End.Line - 1),
									Character: uint32(labelRange.End.Column - 2), // Skip closing quote
								},
							},
							NewText: newName,
						}

						fileURI := lsp.URI(uri.File(filepath.ToSlash(fname)))
						edits[fileURI] = append(edits[fileURI], edit)
					}
				}
			}
		}
		return nil
	})
}

// findAndRenamePathReferences finds all references to paths containing the component
// and creates edits to rename that specific component in the path.
func (s *Server) findAndRenamePathReferences(ctx context.Context, _ string, componentIdx int, oldName, newName string,
	pathComponents []string) map[lsp.DocumentURI][]lsp.TextEdit {

	edits := make(map[lsp.DocumentURI][]lsp.TextEdit)

	_ = filepath.Walk(s.workspace, func(path string, fileInfo os.FileInfo, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil || fileInfo == nil || fileInfo.IsDir() {
			return nil
		}

		// Skip non-Terramate files
		if !isTerramateFile(path) {
			return nil
		}

		// Find and rename path components in this file
		s.renamePathComponentInFile(path, componentIdx, oldName, newName, pathComponents, edits)

		return nil
	})

	return edits
}

// renamePathComponentInFile finds references and renames the specific path component
func (s *Server) renamePathComponentInFile(fname string, componentIdx int, oldName, newName string,
	pathComponents []string, edits map[lsp.DocumentURI][]lsp.TextEdit) {

	content, err := os.ReadFile(fname)
	if err != nil {
		return
	}

	file, diags := hclsyntax.ParseConfig(content, fname, hcl.InitialPos)
	if diags.HasErrors() {
		return
	}

	syntaxBody, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return
	}

	_ = hclsyntax.VisitAll(syntaxBody, func(node hclsyntax.Node) hcl.Diagnostics {
		if expr, ok := node.(hclsyntax.Expression); ok {
			if scopeExpr, ok := expr.(*hclsyntax.ScopeTraversalExpr); ok {
				// Check if this is a global.* reference that matches our path
				if scopeExpr.Traversal.RootName() == "global" && len(scopeExpr.Traversal) > componentIdx+1 {
					// Check if the path matches up to the component we're renaming
					matches := true
					for i := 0; i <= componentIdx && i < len(pathComponents); i++ {
						if i+1 >= len(scopeExpr.Traversal) {
							matches = false
							break
						}
						attr, ok := scopeExpr.Traversal[i+1].(hcl.TraverseAttr)
						if !ok || attr.Name != pathComponents[i] {
							matches = false
							break
						}
					}

					if matches {
						// This reference uses the path component we're renaming
						// Get the specific attribute at the component index
						targetAttr, ok := scopeExpr.Traversal[componentIdx+1].(hcl.TraverseAttr)
						if ok && targetAttr.Name == oldName {
							// Create edit for just this component
							edit := lsp.TextEdit{
								Range: lsp.Range{
									Start: lsp.Position{
										Line:      uint32(targetAttr.SrcRange.Start.Line - 1),
										Character: uint32(targetAttr.SrcRange.Start.Column), // Skip dot
									},
									End: lsp.Position{
										Line:      uint32(targetAttr.SrcRange.End.Line - 1),
										Character: uint32(targetAttr.SrcRange.End.Column - 1),
									},
								},
								NewText: newName,
							}

							fileURI := lsp.URI(uri.File(filepath.ToSlash(fname)))
							edits[fileURI] = append(edits[fileURI], edit)
						}
					}
				}
			}
		}
		return nil
	})
}
