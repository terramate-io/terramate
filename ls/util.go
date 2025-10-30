// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tmls

import (
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/hcl/v2/hclsyntax"
	lsp "go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// isTerramateFile checks if a filename is a Terramate file (.tm or .tm.hcl).
func isTerramateFile(filename string) bool {
	return strings.HasSuffix(filename, ".tm") || strings.HasSuffix(filename, ".tm.hcl")
}

// posInRange checks if a position is within a given range
func posInRange(pos hcl.Pos, r hcl.Range) bool {
	return (r.Start.Line < pos.Line || (r.Start.Line == pos.Line && r.Start.Column <= pos.Column)) &&
		(r.End.Line > pos.Line || (r.End.Line == pos.Line && r.End.Column > pos.Column))
}

// isEnvVarTraversal checks if a traversal is a terramate.run.env.* reference
// Returns true if the traversal matches: terramate.run.env.VAR_NAME
func isEnvVarTraversal(traversal hcl.Traversal) bool {
	if len(traversal) < 4 {
		return false
	}
	if traversal.RootName() != "terramate" {
		return false
	}
	attr1, ok := traversal[1].(hcl.TraverseAttr)
	if !ok || attr1.Name != "run" {
		return false
	}
	attr2, ok := traversal[2].(hcl.TraverseAttr)
	if !ok || attr2.Name != "env" {
		return false
	}
	_, ok = traversal[3].(hcl.TraverseAttr)
	return ok
}

// extractEnvVarName extracts the variable name from a terramate.run.env.VAR_NAME traversal
// Returns empty string if the traversal is not a valid env var reference
func extractEnvVarName(traversal hcl.Traversal) string {
	if !isEnvVarTraversal(traversal) {
		return ""
	}
	if varAttr, ok := traversal[3].(hcl.TraverseAttr); ok {
		return varAttr.Name
	}
	return ""
}

// posToByteOffset converts a line and character position to a byte offset in the content.
// The character position is interpreted as UTF-16 code units (LSP default encoding).
func posToByteOffset(content []byte, line, character int) int {
	currentLine := 0
	currentCol := 0

	for i := 0; i < len(content); {
		if currentLine == line && currentCol == character {
			return i
		}

		if content[i] == '\n' {
			currentLine++
			currentCol = 0
			i++
		} else {
			// Decode the UTF-8 character to get its byte length
			r, size := utf8.DecodeRune(content[i:])
			if r == utf8.RuneError && size == 1 {
				// Invalid UTF-8, treat as single byte
				currentCol++
				i++
			} else {
				// Count UTF-16 code units for this rune
				// Characters >= U+10000 require 2 UTF-16 code units (surrogate pairs)
				if r >= 0x10000 {
					currentCol += 2
				} else {
					currentCol++
				}
				i += size
			}
		}
	}

	return len(content)
}

// searchGlobalsInDirWithPath searches for a global using a full attribute path.
// Supports labeled globals like globals "map" "nested" { key = "value" }
func (s *Server) searchGlobalsInDirWithPath(dir string, attrPath []string) (*lsp.Location, bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, false, err
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
		location, found, err := s.findGlobalInFileWithPath(fullPath, attrPath)
		if err != nil {
			s.log.Debug().Err(err).Str("file", fullPath).Msg("skipping file with errors")
			continue
		}
		if found {
			return location, true, nil
		}
	}

	return nil, false, nil
}

// searchEnvInDir searches for an env variable definition in terramate.config.run.env blocks
func (s *Server) searchEnvInDir(dir string, envVarName string) (*lsp.Location, bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, false, err
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
		location, found, err := s.findEnvInFile(fullPath, envVarName)
		if err != nil {
			s.log.Debug().Err(err).Str("file", fullPath).Msg("skipping file with errors")
			continue
		}
		if found {
			return location, true, nil
		}
	}

	return nil, false, nil
}

// findAllEnvInHierarchy searches for ALL env variable definitions with the given name
// across the entire directory hierarchy from current dir to workspace root.
// Returns all locations where the env var is defined (for renaming all occurrences).
func (s *Server) findAllEnvInHierarchy(fname string, envVarName string) ([]lsp.Location, error) {
	var locations []lsp.Location
	dir := filepath.Dir(fname)

	for {
		entries, err := os.ReadDir(dir)
		if err != nil {
			// Log but continue to parent directory
			s.log.Debug().Err(err).Str("dir", dir).Msg("error reading directory")
		} else {
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}

				filename := entry.Name()
				if !isTerramateFile(filename) {
					continue
				}

				fullPath := filepath.Join(dir, filename)
				location, found, err := s.findEnvInFile(fullPath, envVarName)
				if err != nil {
					s.log.Debug().Err(err).Str("file", fullPath).Msg("skipping file with errors")
					continue
				}
				if found {
					locations = append(locations, *location)
				}
			}
		}

		// Move to parent directory
		parent := filepath.Dir(dir)
		if parent == dir || !strings.HasPrefix(parent, s.workspace) {
			// Reached root or left workspace
			break
		}
		dir = parent
	}

	return locations, nil
}

// findEnvInFile searches for an env variable in terramate.config.run.env block
func (s *Server) findEnvInFile(fname string, envVarName string) (*lsp.Location, bool, error) {
	syntaxBody, err := parseHCLFile(fname)
	if err != nil || syntaxBody == nil {
		return nil, false, err
	}

	var location *lsp.Location
	var found bool

	// Look for terramate.config.run.env blocks
	_ = hclsyntax.VisitAll(syntaxBody, func(node hclsyntax.Node) hcl.Diagnostics {
		if block, ok := node.(*hclsyntax.Block); ok {
			if block.Type == "terramate" {
				// Look for config block inside terramate
				for _, configBlock := range block.Body.Blocks {
					if configBlock.Type == "config" {
						// Look for run block inside config
						for _, runBlock := range configBlock.Body.Blocks {
							if runBlock.Type == "run" {
								// Look for env block inside run
								for _, envBlock := range runBlock.Body.Blocks {
									if envBlock.Type == "env" {
										// Found terramate.config.run.env block
										// Check if our env var is defined here
										for _, attr := range envBlock.Body.Attributes {
											if attr.Name == envVarName {
												location = createAttrLocation(fname, attr)
												found = true
												return nil
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
		return nil
	})

	return location, found, nil
}

// findGlobalInFileWithPath searches for a global attribute using a full path.
// Supports:
//   - Simple attributes: ["my_var"] in globals { my_var = "value" }
//   - Map blocks: ["my_map"] for map "my_map" { ... }
//   - Labeled globals: ["gclz_config", "terraform", "providers", "google", "config", "region"]
//     for globals "gclz_config" "terraform" "providers" "google" "config" { region = "..." }
func (s *Server) findGlobalInFileWithPath(fname string, attrPath []string) (*lsp.Location, bool, error) {
	syntaxBody, err := parseHCLFile(fname)
	if err != nil || syntaxBody == nil {
		return nil, false, err
	}

	var location *lsp.Location
	var found bool

	_ = hclsyntax.VisitAll(syntaxBody, func(node hclsyntax.Node) hcl.Diagnostics {
		if block, ok := node.(*hclsyntax.Block); ok {
			if block.Type == "globals" {
				// Handle labeled globals: globals "map" "nested" { key = "value" }
				if len(block.Labels) > 0 {
					// Check if this labeled block matches our search path
					if matchesLabeledGlobal(block, attrPath) {
						// Found the right labeled block, now find the attribute
						finalAttr := attrPath[len(block.Labels)]
						for _, attr := range block.Body.Attributes {
							if attr.Name == finalAttr {
								location = createAttrLocation(fname, attr)
								found = true
								return nil
							}
						}
					}
					// This labeled block doesn't match, continue searching
					return nil
				}

				// Unlabeled globals block
				rootAttr := attrPath[0]

				// Check attributes in this globals block
				for _, attr := range block.Body.Attributes {
					if attr.Name == rootAttr {
						// If we have nested path (e.g., global.meta.env), try to navigate into the object
						if len(attrPath) > 1 {
							nestedLoc := s.findNestedObjectKey(fname, attr.Expr, attrPath[1:])
							if nestedLoc != nil {
								location = nestedLoc
								found = true
								return nil
							}
						}
						// No nested path or nested key not found, return the root attribute
						location = createAttrLocation(fname, attr)
						found = true
						return nil
					}
				}

				// Check map blocks
				for _, nestedBlock := range block.Body.Blocks {
					if nestedBlock.Type == "map" && len(nestedBlock.Labels) > 0 && nestedBlock.Labels[0] == rootAttr {
						location = &lsp.Location{
							URI: lsp.URI(uri.File(filepath.ToSlash(fname))),
							Range: lsp.Range{
								Start: lsp.Position{
									Line:      uint32(nestedBlock.TypeRange.Start.Line - 1),
									Character: uint32(nestedBlock.TypeRange.Start.Column - 1),
								},
								End: lsp.Position{
									Line:      uint32(nestedBlock.LabelRanges[0].End.Line - 1),
									Character: uint32(nestedBlock.LabelRanges[0].End.Column - 1),
								},
							},
						}
						found = true
						return nil
					}
				}
			}
		}
		return nil
	})

	return location, found, nil
}

// searchStackInDir searches for a stack attribute definition in the given directory.
//
// Performance: Only searches files in the specified directory (non-recursive).
// Returns immediately upon finding the first match.
func (s *Server) searchStackInDir(dir string, attrName string) (*lsp.Location, bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, false, err
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
		location, found, err := s.findStackAttributeInFile(fullPath, attrName)
		if err != nil {
			s.log.Debug().Err(err).Str("file", fullPath).Msg("skipping file with errors")
			continue
		}
		if found {
			return location, true, nil
		}
	}

	return nil, false, nil
}

// findStackAttributeInFile searches for a stack attribute definition in a single file.
//
// Performance: Parses the entire file and visits all blocks. Returns early upon finding
// the first matching stack attribute.
func (s *Server) findStackAttributeInFile(fname string, attrName string) (*lsp.Location, bool, error) {
	syntaxBody, err := parseHCLFile(fname)
	if err != nil || syntaxBody == nil {
		return nil, false, err
	}

	var location *lsp.Location
	var found bool

	_ = hclsyntax.VisitAll(syntaxBody, func(node hclsyntax.Node) hcl.Diagnostics {
		if block, ok := node.(*hclsyntax.Block); ok {
			if block.Type == "stack" {
				// Check attributes in this stack block
				for _, attr := range block.Body.Attributes {
					if attr.Name == attrName {
						location = createAttrLocation(fname, attr)
						found = true
						return nil
					}
				}
			}
		}
		return nil
	})

	return location, found, nil
}

// matchesLabeledGlobal checks if a labeled globals block matches the attribute path
// Example: globals "gclz_config" "terraform" "providers" matches
// path ["gclz_config", "terraform", "providers", "google", "config", "region"]
func matchesLabeledGlobal(block *hclsyntax.Block, attrPath []string) bool {
	// The block's labels must match the beginning of the attrPath
	if len(attrPath) <= len(block.Labels) {
		return false // Path must be longer than labels (need at least one attribute)
	}

	// Check if all labels match
	for i, label := range block.Labels {
		if attrPath[i] != label {
			return false
		}
	}

	return true
}

// createAttrLocation creates an LSP location for an attribute
func createAttrLocation(fname string, attr *hclsyntax.Attribute) *lsp.Location {
	return &lsp.Location{
		URI:   lsp.URI(uri.File(filepath.ToSlash(fname))),
		Range: hclNameRangeToLSP(attr.NameRange),
	}
}

// findNestedObjectKey searches for a nested key within an HCL expression
// Supports:
//   - Direct object literals: { env = "prod", ... }
//   - Objects within function calls: tm_try(..., { env = "prod" })
//
// Returns the location of the nested key if found, nil otherwise
func (s *Server) findNestedObjectKey(fname string, expr hcl.Expression, keyPath []string) *lsp.Location {
	if len(keyPath) == 0 {
		return nil
	}

	targetKey := keyPath[0]

	// Check if this is directly an object constructor
	if objExpr, ok := expr.(*hclsyntax.ObjectConsExpr); ok {
		loc := s.searchObjectForKey(fname, objExpr, targetKey, keyPath)
		if loc != nil {
			return loc
		}
	}

	// Check if this is a function call (search all arguments for objects)
	if funcExpr, ok := expr.(*hclsyntax.FunctionCallExpr); ok {
		for _, arg := range funcExpr.Args {
			loc := s.findNestedObjectKey(fname, arg, keyPath)
			if loc != nil {
				return loc
			}
		}
	}

	return nil
}

// searchObjectForKey searches an object expression for a specific key
func (s *Server) searchObjectForKey(fname string, objExpr *hclsyntax.ObjectConsExpr, targetKey string, keyPath []string) *lsp.Location {
	for _, item := range objExpr.Items {
		// Get the key expression
		if keyExpr, ok := item.KeyExpr.(*hclsyntax.ObjectConsKeyExpr); ok {
			// Check if this is a traversal or literal that matches our target
			if scopeExpr, ok := keyExpr.Wrapped.(*hclsyntax.ScopeTraversalExpr); ok {
				if len(scopeExpr.Traversal) == 1 {
					if root, ok := scopeExpr.Traversal[0].(hcl.TraverseRoot); ok {
						if root.Name == targetKey {
							// Found the key!
							if len(keyPath) == 1 {
								// This is the final key, return its location
								return &lsp.Location{
									URI:   lsp.URI(uri.File(filepath.ToSlash(fname))),
									Range: hclRangeToLSP(root.SrcRange),
								}
							}
							// More keys in path, recursively search in the value expression
							return s.findNestedObjectKey(fname, item.ValueExpr, keyPath[1:])
						}
					}
				}
			}
		}
	}
	return nil
}

// hasLabeledBlockWithPath checks if a labeled block exists with the exact label path
// This is used to determine if path components are labels or nested object keys
func (s *Server) hasLabeledBlockWithPath(fname string, labelPath []string) bool {
	dir := filepath.Dir(fname)

	// Search current directory and parents
	for {
		// Check this directory
		if s.dirHasExactLabeledBlock(dir, labelPath) {
			return true
		}

		// Check imports from this directory
		visited := make(map[string]bool)
		imports := s.collectImportsFromDir(dir, visited)
		for _, importFile := range imports {
			if visited[importFile] {
				continue
			}
			visited[importFile] = true

			if s.fileHasExactLabeledBlock(importFile, labelPath) {
				return true
			}
		}

		// Move to parent directory
		parent := filepath.Dir(dir)
		if parent == dir || !strings.HasPrefix(parent, s.workspace) {
			break
		}
		dir = parent
	}

	return false
}

// dirHasExactLabeledBlock checks if any file in directory has labeled block with exact labels
func (s *Server) dirHasExactLabeledBlock(dir string, labelPath []string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
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
		if s.fileHasExactLabeledBlock(fullPath, labelPath) {
			return true
		}
	}

	return false
}

// fileHasExactLabeledBlock checks if file has globals block with labels matching or prefixing labelPath
func (s *Server) fileHasExactLabeledBlock(fname string, labelPath []string) bool {
	syntaxBody, err := parseHCLFile(fname)
	if err != nil || syntaxBody == nil {
		return false
	}

	found := false
	_ = hclsyntax.VisitAll(syntaxBody, func(node hclsyntax.Node) hcl.Diagnostics {
		if block, ok := node.(*hclsyntax.Block); ok {
			if block.Type == "globals" && len(block.Labels) >= len(labelPath) {
				// Check if block labels START WITH our labelPath
				allMatch := true
				for i, label := range labelPath {
					if block.Labels[i] != label {
						allMatch = false
						break
					}
				}
				if allMatch {
					found = true
					return nil
				}
			}
		}
		return nil
	})

	return found
}

// findAllLabeledBlockDefinitions searches the workspace for all labeled blocks matching the label path
// Returns all locations where labeled blocks with this label path are defined
// Useful for go-to-definition when clicking on labels in paths like global.a.b.c
func (s *Server) findAllLabeledBlockDefinitions(fname string, labelPath []string) []lsp.Location {
	var locations []lsp.Location
	visited := make(map[string]bool)

	// Search from current directory up to workspace root
	dir := filepath.Dir(fname)
	for {
		// Search this directory and its imports
		locs := s.findLabeledBlocksInDirForDefinition(dir, labelPath, visited)
		locations = append(locations, locs...)

		// Move to parent directory
		parent := filepath.Dir(dir)
		if parent == dir || !strings.HasPrefix(parent, s.workspace) {
			break
		}
		dir = parent
	}

	return locations
}

// findLabeledBlocksInDirForDefinition finds labeled block definitions in a directory and its imports
func (s *Server) findLabeledBlocksInDirForDefinition(dir string, labelPath []string, visited map[string]bool) []lsp.Location {
	var locations []lsp.Location

	// Search files in this directory
	entries, err := os.ReadDir(dir)
	if err != nil {
		return locations
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

		// Find labeled blocks in this file
		locs := s.findLabeledBlockLocationsInFile(fullPath, labelPath)
		locations = append(locations, locs...)
	}

	// Also search imported files
	imports := s.collectImportsFromDir(dir, visited)
	for _, importFile := range imports {
		if visited[importFile] {
			continue
		}
		visited[importFile] = true
		locs := s.findLabeledBlockLocationsInFile(importFile, labelPath)
		locations = append(locations, locs...)
	}

	return locations
}

// findLabeledBlockLocationsInFile finds all labeled blocks in a file that match the label path
// Returns the location of the block's opening (the "globals" keyword)
func (s *Server) findLabeledBlockLocationsInFile(fname string, labelPath []string) []lsp.Location {
	var locations []lsp.Location

	syntaxBody, err := parseHCLFile(fname)
	if err != nil || syntaxBody == nil {
		return locations
	}

	_ = hclsyntax.VisitAll(syntaxBody, func(node hclsyntax.Node) hcl.Diagnostics {
		if block, ok := node.(*hclsyntax.Block); ok {
			if block.Type == "globals" && len(block.Labels) >= len(labelPath) {
				// Check if this block's labels start with our label path
				matches := true
				for i, label := range labelPath {
					if block.Labels[i] != label {
						matches = false
						break
					}
				}

				if matches {
					// Found a matching labeled block
					// Return location of the label at the position cursor is on
					labelIdx := len(labelPath) - 1
					labelRange := block.LabelRanges[labelIdx]

					location := lsp.Location{
						URI:   lsp.URI(uri.File(filepath.ToSlash(fname))),
						Range: hclLabelRangeToLSP(labelRange),
					}
					locations = append(locations, location)
				}
			}
		}
		return nil
	})

	return locations
}
