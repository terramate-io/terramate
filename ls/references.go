// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tmls

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
	"github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/hcl/v2/hclsyntax"
	"go.lsp.dev/jsonrpc2"
	lsp "go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func (s *Server) handleReferences(
	ctx context.Context,
	reply jsonrpc2.Replier,
	r jsonrpc2.Request,
	log zerolog.Logger,
) error {
	var params lsp.ReferenceParams
	if err := json.Unmarshal(r.Params(), &params); err != nil {
		log.Error().Err(err).Msg("failed to unmarshal params")
		return jsonrpc2.ErrParse
	}

	fname := params.TextDocument.URI.Filename()
	line := params.Position.Line
	character := params.Position.Character

	log.Debug().
		Str("file", fname).
		Uint32("line", line).
		Uint32("character", character).
		Bool("includeDeclaration", params.Context.IncludeDeclaration).
		Msg("handling find references")

	// Get file content (from cache if available, or disk)
	content, err := s.getDocumentContent(fname)
	if err != nil {
		log.Error().Err(err).Msg("failed to get file content")
		return reply(ctx, nil, nil)
	}

	// Find all references to the symbol at this position
	locations, err := s.findAllReferences(ctx, fname, content, line, character, params.Context.IncludeDeclaration)
	if err != nil {
		log.Debug().Err(err).Msg("failed to find references")
		return reply(ctx, nil, nil)
	}

	return reply(ctx, locations, nil)
}

// findAllReferences finds all references to the symbol at the given position
func (s *Server) findAllReferences(ctx context.Context, fname string, content []byte, line, character uint32, includeDeclaration bool) ([]lsp.Location, error) {
	// Parse the file to find what symbol is at the cursor
	syntaxBody, err := parseHCLContent(content, fname)
	if err != nil {
		return nil, err
	}
	if syntaxBody == nil {
		return nil, nil
	}

	targetPos := hcl.Pos{
		Line:   int(line) + 1,
		Column: int(character) + 1,
		Byte:   posToByteOffset(content, int(line), int(character)),
	}

	// Find the symbol at cursor position
	symbolInfo := s.findSymbolAtPosition(syntaxBody, targetPos)
	if symbolInfo == nil {
		return nil, nil
	}

	// Search for all references to this symbol
	var locations []lsp.Location

	// Include the definition if requested
	if includeDeclaration {
		defLoc := s.findAndReturnDefinition(fname, symbolInfo)
		if defLoc != nil {
			locations = append(locations, *defLoc)
		}
	}

	// Search for references to this symbol
	var references []lsp.Location
	if symbolInfo.namespace == "let" {
		// Let variables are file-scoped, only search current file
		references = s.findReferencesInFile(fname, symbolInfo)
	} else {
		// Search entire workspace for global, terramate.stack, etc.
		references = s.searchReferencesInWorkspace(ctx, symbolInfo)
	}
	locations = append(locations, references...)

	return locations, nil
}

// symbolInfo contains information about a symbol for reference finding
type symbolInfo struct {
	namespace        string   // "global", "let", "terramate.stack", or other "terramate.*"
	attributeName    string   // e.g., "my_var" for global.my_var, "name" for terramate.stack.name
	fullPath         string   // e.g., "global.my_var" or "terramate.stack.name"
	pathComponents   []string // Full path split: ["gclz_config", "terraform", "providers"]
	componentIndex   int      // Which component (0-based): 0="gclz_config", 1="terraform", etc.
	isLabelComponent bool     // True if this is a label in a labeled block, not a final attribute
}

// findSymbolAtPosition identifies what symbol is at the cursor position
func (s *Server) findSymbolAtPosition(body *hclsyntax.Body, targetPos hcl.Pos) *symbolInfo {
	var info *symbolInfo

	s.log.Debug().
		Int("line", targetPos.Line).
		Int("column", targetPos.Column).
		Msg("findSymbolAtPosition: searching for symbol at position")

	// First, check if we're on an attribute definition
	_ = hclsyntax.VisitAll(body, func(node hclsyntax.Node) hcl.Diagnostics {
		if block, ok := node.(*hclsyntax.Block); ok {
			// Check globals block
			if block.Type == "globals" {
				// Build path prefix from block labels
				// For globals "a" "b" "c" { region = "..." }, prefix is "global.a.b.c."
				var pathPrefix string
				if len(block.Labels) > 0 {
					pathPrefix = "global." + strings.Join(block.Labels, ".") + "."
				} else {
					pathPrefix = "global."
				}

				for _, attr := range block.Body.Attributes {
					if posInRange(targetPos, attr.NameRange) {
						// Build pathComponents from labels + attribute name
						pathComponents := make([]string, 0, len(block.Labels)+1)
						pathComponents = append(pathComponents, block.Labels...)
						pathComponents = append(pathComponents, attr.Name)

						info = &symbolInfo{
							namespace:      "global",
							attributeName:  attr.Name,
							fullPath:       pathPrefix + attr.Name,
							pathComponents: pathComponents,
						}
						return nil
					}
				}
				// Check map blocks in globals
				for _, nested := range block.Body.Blocks {
					if nested.Type == "map" {
						// Check if cursor is on the map block label itself
						if len(nested.Labels) > 0 && len(nested.LabelRanges) > 0 {
							if posInRange(targetPos, nested.LabelRanges[0]) {
								// Build pathComponents from labels + map block label
								pathComponents := make([]string, 0, len(block.Labels)+1)
								pathComponents = append(pathComponents, block.Labels...)
								pathComponents = append(pathComponents, nested.Labels[0])

								info = &symbolInfo{
									namespace:      "global",
									attributeName:  nested.Labels[0],
									fullPath:       pathPrefix + nested.Labels[0],
									pathComponents: pathComponents,
								}
								return nil
							}
						}

						// Check attributes inside the map block
						for _, attr := range nested.Body.Attributes {
							if posInRange(targetPos, attr.NameRange) {
								// Build pathComponents from labels + attribute name
								pathComponents := make([]string, 0, len(block.Labels)+1)
								pathComponents = append(pathComponents, block.Labels...)
								pathComponents = append(pathComponents, attr.Name)

								info = &symbolInfo{
									namespace:      "global",
									attributeName:  attr.Name,
									fullPath:       pathPrefix + attr.Name,
									pathComponents: pathComponents,
								}
								return nil
							}
						}
					}
				}
			}

			// Check stack block attributes (for definition location only)
			// Note: Direct stack.* namespace is not supported, only terramate.stack.*
			if block.Type == "stack" {
				for _, attr := range block.Body.Attributes {
					if posInRange(targetPos, attr.NameRange) {
						// When cursor is on a stack attribute definition,
						// normalize to terramate.stack.* namespace for consistency
						info = &symbolInfo{
							namespace:     "terramate.stack",
							attributeName: attr.Name,
							fullPath:      "terramate.stack." + attr.Name,
						}
						return nil
					}
				}
			}

			// Check lets block
			if block.Type == "generate_hcl" || block.Type == "generate_file" {
				for _, nested := range block.Body.Blocks {
					if nested.Type == "lets" {
						for _, attr := range nested.Body.Attributes {
							if posInRange(targetPos, attr.NameRange) {
								info = &symbolInfo{
									namespace:     "let",
									attributeName: attr.Name,
									fullPath:      "let." + attr.Name,
								}
								return nil
							}
						}
					}
				}
			}

			// Check terramate.config.run.env blocks
			if block.Type == "terramate" {
				for _, configBlock := range block.Body.Blocks {
					if configBlock.Type == "config" {
						for _, runBlock := range configBlock.Body.Blocks {
							if runBlock.Type == "run" {
								for _, envBlock := range runBlock.Body.Blocks {
									if envBlock.Type == "env" {
										// Check attributes in env block
										for _, attr := range envBlock.Body.Attributes {
											if posInRange(targetPos, attr.NameRange) {
												// Use terramate.run.env as the namespace to match references
												info = &symbolInfo{
													namespace:     "terramate.run.env",
													attributeName: attr.Name,
													fullPath:      "terramate.run.env." + attr.Name,
												}
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

	if info != nil {
		s.log.Debug().
			Str("namespace", info.namespace).
			Str("attributeName", info.attributeName).
			Str("fullPath", info.fullPath).
			Strs("pathComponents", info.pathComponents).
			Int("componentIndex", info.componentIndex).
			Bool("isLabelComponent", info.isLabelComponent).
			Msg("findSymbolAtPosition: found symbol on definition")
		return info
	}

	// If not on a definition, check if we're on a reference
	_ = hclsyntax.VisitAll(body, func(node hclsyntax.Node) hcl.Diagnostics {
		if expr, ok := node.(hclsyntax.Expression); ok {
			exprRange := expr.Range()
			if posInRange(targetPos, exprRange) {
				// Check for scope traversal (e.g., global.my_var)
				if scopeExpr, ok := expr.(*hclsyntax.ScopeTraversalExpr); ok {
					if len(scopeExpr.Traversal) >= 2 {
						rootName := scopeExpr.Traversal.RootName()

						// Reject direct stack.* references (not supported)
						if rootName == "stack" {
							return nil
						}

						// For labeled globals like global.a.b.c.d.region
						// The attribute name is the LAST part of the traversal
						var attrName string
						var fullPathParts []string

						for i := 1; i < len(scopeExpr.Traversal); i++ {
							traverseAttr, ok := scopeExpr.Traversal[i].(hcl.TraverseAttr)
							if !ok {
								return nil
							}
							fullPathParts = append(fullPathParts, traverseAttr.Name)
						}

						if len(fullPathParts) == 0 {
							return nil
						}

						// The attribute name is the last part
						attrName = fullPathParts[len(fullPathParts)-1]
						fullPath := rootName + "." + strings.Join(fullPathParts, ".")

						// Determine which component index cursor is on
						componentIdx := -1
						for i, part := range scopeExpr.Traversal {
							if i == 0 {
								continue // Skip root
							}
							if attr, ok := part.(hcl.TraverseAttr); ok {
								if posInRange(targetPos, attr.SrcRange) {
									componentIdx = i - 1 // 0-based index in fullPathParts
									break
								}
							}
						}

						// If we couldn't determine the component (shouldn't happen), default to last
						if componentIdx < 0 && len(fullPathParts) > 0 {
							componentIdx = len(fullPathParts) - 1
						}

						// Mark as potential label component if not the final attribute
						// We'll verify it's actually a label when needed (in rename logic)
						isLabel := componentIdx >= 0 && componentIdx < len(fullPathParts)-1

						info = &symbolInfo{
							namespace:        rootName,
							attributeName:    attrName,
							fullPath:         fullPath,
							pathComponents:   fullPathParts,
							componentIndex:   componentIdx,
							isLabelComponent: isLabel,
						}

						s.log.Debug().
							Str("namespace", rootName).
							Str("attributeName", attrName).
							Str("fullPath", fullPath).
							Strs("pathComponents", fullPathParts).
							Int("componentIndex", componentIdx).
							Bool("isLabelComponent", isLabel).
							Msg("findSymbolAtPosition: found symbol on reference")

						// Handle terramate.stack.* - special case
						if rootName == "terramate" && len(fullPathParts) >= 2 && fullPathParts[0] == "stack" {
							// For terramate.stack.name, the attribute is "name"
							info.namespace = "terramate.stack"
							info.attributeName = fullPathParts[1]
							info.fullPath = "terramate.stack." + fullPathParts[1]
						}

						// Handle terramate.run.env.* - special case for environment variables
						if rootName == "terramate" && len(fullPathParts) >= 3 && fullPathParts[0] == "run" && fullPathParts[1] == "env" {
							// For terramate.run.env.FOO, the attribute is "FOO"
							info.namespace = "terramate.run.env"
							info.attributeName = fullPathParts[2]
							info.fullPath = "terramate.run.env." + fullPathParts[2]
						}
					}
				}
			}
		}
		return nil
	})

	return info
}

// findAndReturnDefinition finds the definition of a symbol and returns its location
// Now uses import-aware search for globals
func (s *Server) findAndReturnDefinition(fname string, info *symbolInfo) *lsp.Location {
	switch info.namespace {
	case "global":
		// Extract full path from fullPath string
		// For "global.gclz_config.terraform.providers.google.config.region"
		// We need ["gclz_config", "terraform", "providers", "google", "config", "region"]
		fullPath := strings.TrimPrefix(info.fullPath, "global.")
		attrPath := strings.Split(fullPath, ".")

		// Use import-aware search for globals (follows import chains)
		location, _ := s.findGlobalWithImports(fname, attrPath)
		return location

	case "terramate.stack":
		// Search current and parent directories for stack attributes
		dir := filepath.Dir(fname)
		for {
			location, found, _ := s.searchStackInDir(dir, info.attributeName)
			if found {
				return location
			}
			parent := filepath.Dir(dir)
			if parent == dir || !strings.HasPrefix(parent, s.workspace) {
				break
			}
			dir = parent
		}

	case "let":
		// Let variables are in the same file
		body, err := parseHCLFile(fname)
		if err != nil || body == nil {
			return nil
		}
		return s.findDefinitionLocation(body, info, fname)

	case "terramate.run.env":
		// Environment variables are always defined in terramate.tm.hcl at the workspace root
		// in the terramate.config.run.env block
		terramateConfigPath := filepath.Join(s.workspace, "terramate.tm.hcl")
		location, found, err := s.findEnvInFile(terramateConfigPath, info.attributeName)
		if err != nil {
			s.log.Debug().Err(err).Str("file", terramateConfigPath).Msg("failed to read env definition file")
			return nil
		}
		if found {
			return location
		}
	}

	return nil
}

// findDefinitionLocation finds the exact location of a symbol's definition in a given file
func (s *Server) findDefinitionLocation(body *hclsyntax.Body, info *symbolInfo, fname string) *lsp.Location {
	var location *lsp.Location

	_ = hclsyntax.VisitAll(body, func(node hclsyntax.Node) hcl.Diagnostics {
		if block, ok := node.(*hclsyntax.Block); ok {
			switch info.namespace {
			case "global":
				if block.Type == "globals" {
					for _, attr := range block.Body.Attributes {
						if attr.Name == info.attributeName {
							location = createAttrLocation(fname, attr)
							return nil
						}
					}
					// Check map blocks
					for _, nested := range block.Body.Blocks {
						if nested.Type == "map" {
							// Check if looking for the map block label itself
							if len(nested.Labels) > 0 && nested.Labels[0] == info.attributeName {
								location = &lsp.Location{
									URI:   lsp.URI(uri.File(filepath.ToSlash(fname))),
									Range: hclLabelRangeToLSP(nested.LabelRanges[0]),
								}
								return nil
							}

							// Check attributes inside the map block
							for _, attr := range nested.Body.Attributes {
								if attr.Name == info.attributeName {
									location = createAttrLocation(fname, attr)
									return nil
								}
							}
						}
					}
				}
			case "terramate.stack":
				if block.Type == "stack" {
					for _, attr := range block.Body.Attributes {
						if attr.Name == info.attributeName {
							location = createAttrLocation(fname, attr)
							return nil
						}
					}
				}
			case "let":
				if block.Type == "generate_hcl" || block.Type == "generate_file" {
					for _, nested := range block.Body.Blocks {
						if nested.Type == "lets" {
							for _, attr := range nested.Body.Attributes {
								if attr.Name == info.attributeName {
									location = createAttrLocation(fname, attr)
									return nil
								}
							}
						}
					}
				}
			}
		}
		return nil
	})

	return location
}

// searchReferencesInWorkspace searches all Terramate files for references to the symbol.
//
// Performance Characteristics:
//   - Uses filepath.Walk to traverse the entire workspace directory tree
//   - Skips hidden directories (starting with .), VCS directories (.git, .svn, .hg),
//     dependency directories (node_modules, vendor), and build/cache directories
//   - Only parses files ending in .tm or .tm.hcl
//   - Time complexity: O(n) where n is the number of eligible files
//   - Space complexity: O(m) where m is the number of references found
//
// Performance Notes:
//   - For large workspaces (1000+ files), this may take several seconds
//   - Malformed files are silently skipped (no errors returned)
//   - No caching is performed; each call re-scans the workspace
//   - Consider using .cursorignore or .gitignore to reduce scan scope
//   - Context cancellation is supported for long-running searches
//
// Optimization Opportunities:
//   - Add file caching to avoid re-parsing unchanged files
//   - Use concurrent workers for parallel file processing
func (s *Server) searchReferencesInWorkspace(ctx context.Context, info *symbolInfo) []lsp.Location {
	var locations []lsp.Location

	if s.workspace == "" {
		return locations
	}

	_ = filepath.Walk(s.workspace, func(path string, fileInfo os.FileInfo, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			return nil // Skip files with errors
		}

		if fileInfo == nil {
			return nil
		}

		if fileInfo.IsDir() {
			base := filepath.Base(path)

			// Skip hidden directories (starting with .)
			if base != "." && strings.HasPrefix(base, ".") {
				return filepath.SkipDir
			}

			// Skip common dependency, build, and cache directories
			switch base {
			case "node_modules", "vendor", ".terraform",
				"dist", "build", "out", "target",
				".cache", ".pytest_cache", "__pycache__":
				return filepath.SkipDir
			}

			return nil
		}

		if !isTerramateFile(path) {
			return nil
		}

		refs := s.findReferencesInFile(path, info)
		locations = append(locations, refs...)

		return nil
	})

	return locations
}

// findReferencesInFile finds all references to the symbol in a single file.
//
// Performance: This function parses the entire file using HCL parser and visits all AST nodes.
// Parsing is the most expensive operation. Returns early if file cannot be read or parsed.
func (s *Server) findReferencesInFile(fname string, info *symbolInfo) []lsp.Location {
	syntaxBody, err := parseHCLFile(fname)
	if err != nil || syntaxBody == nil {
		return nil
	}

	var locations []lsp.Location

	_ = hclsyntax.VisitAll(syntaxBody, func(node hclsyntax.Node) hcl.Diagnostics {
		if expr, ok := node.(hclsyntax.Expression); ok {
			// Check for matching scope traversal
			if scopeExpr, ok := expr.(*hclsyntax.ScopeTraversalExpr); ok {
				if s.matchesSymbol(scopeExpr.Traversal, info) {
					// Found a reference - return range of ONLY the last attribute (the renameable part)
					// For global.gclz_project_id, we want to return range of just "gclz_project_id"
					// This way rename replaces "gclz_project_id" → "new_name", keeping "global."

					lastIdx := len(scopeExpr.Traversal) - 1
					if lastIdx < 1 {
						return nil // Skip invalid traversals
					}

					lastAttr, ok := scopeExpr.Traversal[lastIdx].(hcl.TraverseAttr)
					if !ok {
						return nil // Skip non-attribute traversals
					}

					location := lsp.Location{
						URI:   lsp.URI(uri.File(filepath.ToSlash(fname))),
						Range: hclTraverseAttrToLSP(lastAttr.SrcRange),
					}
					locations = append(locations, location)
				}
			}
		}
		return nil
	})

	return locations
}

// matchesSymbol checks if a traversal matches the symbol we're looking for
func (s *Server) matchesSymbol(traversal hcl.Traversal, info *symbolInfo) bool {
	if len(traversal) < 2 {
		return false
	}

	rootName := traversal.RootName()

	// Reject direct stack.* references (only terramate.stack.* is valid)
	if rootName == "stack" {
		return false
	}

	// Handle terramate.stack.* references
	if rootName == "terramate" && len(traversal) >= 3 {
		if attr, ok := traversal[1].(hcl.TraverseAttr); ok && attr.Name == "stack" {
			// info.namespace should be "terramate.stack" (normalized)
			if info.namespace == "terramate.stack" {
				stackTraverseAttr, ok := traversal[2].(hcl.TraverseAttr)
				if !ok {
					return false
				}
				stackAttr := stackTraverseAttr.Name
				return stackAttr == info.attributeName
			}
		}

		// Handle terramate.run.env.* references
		if attr, ok := traversal[1].(hcl.TraverseAttr); ok && attr.Name == "run" {
			if len(traversal) >= 4 {
				if envAttr, ok := traversal[2].(hcl.TraverseAttr); ok && envAttr.Name == "env" {
					// info.namespace should be "terramate.run.env" (normalized)
					if info.namespace == "terramate.run.env" {
						varAttr, ok := traversal[3].(hcl.TraverseAttr)
						if !ok {
							return false
						}
						return varAttr.Name == info.attributeName
					}
				}
			}
		}

		return false
	}

	// Direct namespace match (for global, let, etc.)
	if rootName != info.namespace {
		return false
	}

	// For paths with multiple components (e.g., global.a.b.c.d.region)
	// we need to match the entire path, not just the first component
	if len(info.pathComponents) > 0 {
		// Extract path components from traversal
		var traversalPath []string
		for i := 1; i < len(traversal); i++ {
			if attr, ok := traversal[i].(hcl.TraverseAttr); ok {
				traversalPath = append(traversalPath, attr.Name)
			}
		}

		// Check if the paths match
		// For labeled globals, the traversal path should match the full path
		if len(traversalPath) != len(info.pathComponents) {
			return false
		}

		for i, component := range info.pathComponents {
			if traversalPath[i] != component {
				return false
			}
		}

		return true
	}

	// Fallback for simple paths (backward compatibility)
	traverseAttr, ok := traversal[1].(hcl.TraverseAttr)
	if !ok {
		return false
	}
	attrName := traverseAttr.Name
	return attrName == info.attributeName
}
