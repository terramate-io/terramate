// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tmls

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/hcl/v2/hclsyntax"
	lsp "go.lsp.dev/protocol"
)

// findGlobalWithImports searches for a global definition including imported files
// This handles the case where globals are defined in imported modules
func (s *Server) findGlobalWithImports(fname string, attrPath []string) (*lsp.Location, error) {
	// Keep track of visited files to avoid circular imports
	visited := make(map[string]bool)

	// Start search from current file's directory
	dir := filepath.Dir(fname)

	// Search current directory and parents (including their imports)
	for {
		location, found := s.searchWithImports(dir, attrPath, visited)
		if found {
			return location, nil
		}

		// Move to parent directory
		parent := filepath.Dir(dir)
		if parent == dir || !strings.HasPrefix(parent, s.workspace) {
			break
		}
		dir = parent
	}

	return nil, nil
}

// searchWithImports searches a directory and its imports for a global
func (s *Server) searchWithImports(dir string, attrPath []string, visited map[string]bool) (*lsp.Location, bool) {
	// First search the directory itself
	location, found, err := s.searchGlobalsInDirWithPath(dir, attrPath)
	if err == nil && found {
		return location, true
	}

	// Then search imports from files in this directory (with full recursion)
	location, found = s.searchImportedFiles(dir, attrPath, visited)
	if found {
		return location, true
	}

	return nil, false
}

// searchImportedFiles recursively searches imported files for a global
func (s *Server) searchImportedFiles(dir string, attrPath []string, visited map[string]bool) (*lsp.Location, bool) {
	imports := s.collectImportsFromDir(dir, visited)

	for _, importedFile := range imports {
		if visited[importedFile] {
			continue // Skip circular imports
		}
		visited[importedFile] = true

		// Search the imported file itself
		location, found, err := s.findGlobalInFileWithPath(importedFile, attrPath)
		if err == nil && found {
			return location, true
		}

		// Extract imports from this specific file and search them recursively
		nestedImports := s.extractImportsFromFile(importedFile)

		for _, nestedImport := range nestedImports {
			if visited[nestedImport] {
				continue
			}
			visited[nestedImport] = true

			location, found, err := s.findGlobalInFileWithPath(nestedImport, attrPath)
			if err == nil && found {
				return location, true
			}

			// Continue recursively
			nestedDir := filepath.Dir(nestedImport)
			location, found = s.searchImportedFiles(nestedDir, attrPath, visited)
			if found {
				return location, true
			}
		}
	}

	return nil, false
}

// collectImportsFromDir finds all import statements in .tm and .tm.hcl files in a directory
func (s *Server) collectImportsFromDir(dir string, visited map[string]bool) []string {
	var importedFiles []string

	entries, err := os.ReadDir(dir)
	if err != nil {
		return importedFiles
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

		imports := s.extractImportsFromFile(fullPath)
		importedFiles = append(importedFiles, imports...)
	}

	return importedFiles
}

// extractImportsFromFile extracts all import source paths from a file.
func (s *Server) extractImportsFromFile(fname string) []string {
	var importPaths []string

	content, err := os.ReadFile(fname)
	if err != nil {
		s.log.Debug().Err(err).Str("file", fname).Msg("failed to read file for import extraction")
		return importPaths
	}

	file, diags := hclsyntax.ParseConfig(content, fname, hcl.InitialPos)
	if diags.HasErrors() {
		s.log.Debug().Err(diags).Str("file", fname).Msg("failed to parse file for import extraction")
		return importPaths
	}

	syntaxBody, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return importPaths
	}

	_ = hclsyntax.VisitAll(syntaxBody, func(node hclsyntax.Node) hcl.Diagnostics {
		if block, ok := node.(*hclsyntax.Block); ok {
			if block.Type == "import" {
				if sourceAttr, ok := block.Body.Attributes["source"]; ok {
					var sourcePath string

					// Extract the source path
					if litExpr, ok := sourceAttr.Expr.(*hclsyntax.TemplateExpr); ok {
						if len(litExpr.Parts) == 1 {
							if lit, ok := litExpr.Parts[0].(*hclsyntax.LiteralValueExpr); ok {
								sourcePath = lit.Val.AsString()
							}
						}
					}

					if sourcePath != "" && !strings.Contains(sourcePath, "*") {
						// Resolve the import path to absolute file path
						resolvedPath := s.resolveImportToAbsPath(fname, sourcePath)
						if resolvedPath != "" {
							importPaths = append(importPaths, resolvedPath)
						}
					}
				}
			}
		}
		return nil
	})

	return importPaths
}

// resolveImportToAbsPath resolves an import source path to an absolute file path.
func (s *Server) resolveImportToAbsPath(currentFile string, sourcePath string) string {
	var absPath string

	if filepath.IsAbs(sourcePath) || strings.HasPrefix(sourcePath, "/") {
		absPath = filepath.Join(s.workspace, strings.TrimPrefix(sourcePath, "/"))
	} else {
		currentDir := filepath.Dir(currentFile)
		absPath = filepath.Join(currentDir, sourcePath)
	}

	resolvedPath, err := filepath.Abs(absPath)
	if err != nil {
		s.log.Debug().Err(err).Str("sourcePath", sourcePath).Msg("failed to resolve import path to absolute path")
		return ""
	}

	if s.workspace != "" && !strings.HasPrefix(resolvedPath, s.workspace) {
		s.log.Debug().
			Str("sourcePath", sourcePath).
			Str("resolvedPath", resolvedPath).
			Str("workspace", s.workspace).
			Msg("import path resolves outside workspace - possible directory traversal attempt")
		return ""
	}

	if _, err := os.Stat(resolvedPath); err != nil {
		s.log.Debug().Err(err).Str("path", resolvedPath).Msg("import path does not exist")
		return ""
	}

	return resolvedPath
}
