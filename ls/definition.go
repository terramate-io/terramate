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

func (s *Server) handleDefinition(
	ctx context.Context,
	reply jsonrpc2.Replier,
	r jsonrpc2.Request,
	log zerolog.Logger,
) error {
	log.Debug().Msg("handleDefinition: ENTRY POINT")

	var params lsp.DefinitionParams
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
		Msg("handleDefinition: parameters parsed")

	// Get file content (from cache if available, or disk)
	log.Debug().Msg("handleDefinition: about to get file content")
	content, err := s.getDocumentContent(fname)
	if err != nil {
		log.Error().Err(err).Msg("failed to get file content")
		return reply(ctx, nil, nil)
	}
	log.Debug().Int("contentSize", len(content)).Msg("handleDefinition: got file content")

	// Find the definition location(s) - can return single location or array of locations
	log.Debug().Msg("handleDefinition: calling findDefinitions")
	defLocations, err := s.findDefinitions(fname, content, line, character)
	log.Debug().Int("numLocations", len(defLocations)).Msg("handleDefinition: findDefinitions returned")
	if err != nil {
		log.Debug().Err(err).Msg("failed to find definition")
		return reply(ctx, nil, nil)
	}

	if len(defLocations) == 0 {
		// No definition found
		return reply(ctx, nil, nil)
	}

	// Return single location or array based on count
	// LSP spec supports both Location and Location[]
	if len(defLocations) == 1 {
		return reply(ctx, defLocations[0], nil)
	}

	return reply(ctx, defLocations, nil)
}

// findDefinitions finds the definition location(s) for the symbol at the given position
// Returns a slice of locations - can be multiple for labeled blocks defined across hierarchy levels
func (s *Server) findDefinitions(fname string, content []byte, line, character uint32) ([]lsp.Location, error) {
	s.log.Debug().
		Str("file", fname).
		Uint32("line", line).
		Uint32("character", character).
		Msg("findDefinitions: ENTRY POINT")

	// Parse the HCL file
	syntaxBody, err := parseHCLContent(content, fname)
	if err != nil {
		s.log.Debug().Err(err).Msg("findDefinitions: HCL parse error")
		return nil, err
	}
	if syntaxBody == nil {
		s.log.Debug().Msg("findDefinitions: not a syntax body")
		return nil, nil
	}
	s.log.Debug().Msg("findDefinitions: HCL parsed successfully")

	// Convert LSP position to HCL position (LSP is 0-indexed, HCL is 1-indexed)
	targetPos := hcl.Pos{
		Line:   int(line) + 1,
		Column: int(character) + 1,
		Byte:   posToByteOffset(content, int(line), int(character)),
	}
	s.log.Debug().
		Int("hclLine", targetPos.Line).
		Int("hclColumn", targetPos.Column).
		Int("hclByte", targetPos.Byte).
		Msg("findDefinitions: converted position to HCL")

	// Find what's at this position
	var foundTraversal hcl.Traversal

	// First check if we're in a string literal (for import sources or stack paths)
	stringLocation := s.findStringLiteralDefinition(syntaxBody, targetPos, fname)
	if stringLocation != nil {
		s.log.Debug().Msg("findDefinitions: found string literal definition")
		return []lsp.Location{*stringLocation}, nil
	}

	// Check if we're on a label in a labeled globals block definition
	labelLocations := s.findLabelDefinitionsAtPosition(syntaxBody, targetPos, fname)
	if len(labelLocations) > 0 {
		s.log.Debug().Int("count", len(labelLocations)).Msg("findDefinitions: found label definitions")
		return labelLocations, nil
	}

	s.log.Debug().Msg("findDefinitions: about to walk AST to find traversal")

	// Walk the file to find the expression at the target position
	_ = hclsyntax.VisitAll(syntaxBody, func(node hclsyntax.Node) hcl.Diagnostics {
		if expr, ok := node.(hclsyntax.Expression); ok {
			exprRange := expr.Range()
			if posInRange(targetPos, exprRange) {
				// Check if it's a scope traversal (variable reference)
				if scopeExpr, ok := expr.(*hclsyntax.ScopeTraversalExpr); ok {
					// Always use full traversal - findGlobalDefinitions will handle cursor position
					foundTraversal = scopeExpr.Traversal
				}
				// Check if it's a relative traversal
				if relExpr, ok := expr.(*hclsyntax.RelativeTraversalExpr); ok {
					// Get the full traversal path
					if scopeExpr, ok := relExpr.Source.(*hclsyntax.ScopeTraversalExpr); ok {
						// Combine the source and relative traversals
						fullTraversal := append(hcl.Traversal{}, scopeExpr.Traversal...)
						fullTraversal = append(fullTraversal, relExpr.Traversal...)
						// Always use full traversal - findGlobalDefinitions will handle cursor position
						foundTraversal = fullTraversal
					}
				}
			}
		}
		return nil
	})

	s.log.Debug().Int("traversalLen", len(foundTraversal)).Msg("findDefinitions: finished AST walk")

	if len(foundTraversal) == 0 {
		s.log.Debug().Msg("findDefinitions: no traversal found at position")
		return nil, nil
	}

	// Get the root namespace and attribute path
	rootName := foundTraversal.RootName()
	s.log.Debug().Str("rootName", rootName).Msg("findDefinitions: found traversal")

	// Handle different namespaces
	switch rootName {
	case "global":
		return s.findGlobalDefinitions(fname, content, targetPos, foundTraversal)
	case "let":
		loc, err := s.findLetDefinition(fname, foundTraversal)
		if err != nil || loc == nil {
			return nil, err
		}
		return []lsp.Location{*loc}, nil
	case "terramate":
		// Special handling for env variables - return ALL definitions across hierarchy
		if envVarName := extractEnvVarName(foundTraversal); envVarName != "" {
			allLocations, err := s.findAllEnvInHierarchy(fname, envVarName)
			if err != nil {
				return nil, err
			}
			if len(allLocations) > 0 {
				return allLocations, nil
			}
			return nil, nil
		}

		// For other terramate.* references, return single definition
		loc, err := s.findTerramateDefinition(fname, foundTraversal)
		if err != nil || loc == nil {
			return nil, err
		}
		return []lsp.Location{*loc}, nil
	default:
		// Note: stack.* is not a valid namespace, only terramate.stack.* is supported
		return nil, nil
	}
}

// findGlobalDefinitions finds the definition(s) of a global variable
// Supports cursor-aware navigation: clicking on different parts of the path navigates to different definitions
// Can return multiple locations for labeled blocks defined across hierarchy levels
func (s *Server) findGlobalDefinitions(fname string, _ []byte, targetPos hcl.Pos, traversal hcl.Traversal) ([]lsp.Location, error) {
	if len(traversal) < 2 {
		s.log.Debug().Msg("findGlobalDefinitions: traversal too short")
		return nil, nil
	}

	// Extract the full path from traversal
	var attrPath []string
	for i := 1; i < len(traversal); i++ {
		if attr, ok := traversal[i].(hcl.TraverseAttr); ok {
			attrPath = append(attrPath, attr.Name)
		}
	}

	if len(attrPath) == 0 {
		s.log.Debug().Msg("findGlobalDefinitions: attrPath is empty")
		return nil, nil
	}

	s.log.Debug().
		Strs("attrPath", attrPath).
		Msg("findGlobalDefinitions: extracted path from traversal")

	// Determine which component the cursor is on
	cursorComponentIdx := -1
	for i := 1; i < len(traversal); i++ {
		if attr, ok := traversal[i].(hcl.TraverseAttr); ok {
			if posInRange(targetPos, attr.SrcRange) {
				cursorComponentIdx = i - 1 // 0-based index in attrPath
				s.log.Debug().
					Int("componentIdx", cursorComponentIdx).
					Str("componentName", attrPath[cursorComponentIdx]).
					Msg("findGlobalDefinitions: cursor on component")
				break
			}
		}
	}

	// Check if cursor is on a label component (not the final attribute)
	isOnLabelComponent := cursorComponentIdx >= 0 && cursorComponentIdx < len(attrPath)-1

	s.log.Debug().
		Int("cursorComponentIdx", cursorComponentIdx).
		Bool("isOnLabelComponent", isOnLabelComponent).
		Msg("findGlobalDefinitions: checking if on label component")

	if isOnLabelComponent {
		// Build the label path up to and including the component cursor is on
		labelPath := attrPath[:cursorComponentIdx+1]

		s.log.Debug().
			Strs("labelPath", labelPath).
			Msg("findGlobalDefinitions: checking for labeled blocks")

		// Check if this is actually a labeled block (not nested object)
		if s.hasLabeledBlockWithPath(fname, labelPath) {
			s.log.Debug().Msg("findGlobalDefinitions: found labeled blocks, searching for definitions")
			// Find all labeled block definitions for this label path
			locations := s.findAllLabeledBlockDefinitions(fname, labelPath)
			s.log.Debug().
				Int("count", len(locations)).
				Msg("findGlobalDefinitions: found labeled block definitions")
			if len(locations) > 0 {
				return locations, nil
			}
		} else {
			s.log.Debug().Msg("findGlobalDefinitions: no labeled blocks found with this path")
			// Not a labeled block - it's a nested object attribute
			// Search for just this component (not the full nested path)
			// This handles cases like cursor on "gclz_meta" in "global.gclz_meta.env"
			location, err := s.findGlobalWithImports(fname, labelPath)
			if err != nil {
				return nil, err
			}
			if location != nil {
				return []lsp.Location{*location}, nil
			}
		}
	}

	// Cursor is on the final attribute - treat as attribute navigation with full path
	// This handles cases like cursor on "env" in "global.gclz_meta.env"
	// Search for the global definition including imports
	location, err := s.findGlobalWithImports(fname, attrPath)
	if err != nil {
		return nil, err
	}

	if location == nil {
		return nil, nil
	}

	return []lsp.Location{*location}, nil
}

// findLetDefinition finds the definition of a let variable
func (s *Server) findLetDefinition(fname string, traversal hcl.Traversal) (*lsp.Location, error) {
	if len(traversal) < 2 {
		return nil, nil
	}

	// Get the let attribute name (let.something)
	attrTraverse, ok := traversal[1].(hcl.TraverseAttr)
	if !ok {
		return nil, nil
	}
	attrName := attrTraverse.Name

	// Lets are defined in generate_hcl and generate_file blocks
	syntaxBody, err := parseHCLFile(fname)
	if err != nil || syntaxBody == nil {
		return nil, err
	}

	var location *lsp.Location

	_ = hclsyntax.VisitAll(syntaxBody, func(node hclsyntax.Node) hcl.Diagnostics {
		if block, ok := node.(*hclsyntax.Block); ok {
			if block.Type == "generate_hcl" || block.Type == "generate_file" {
				// Look for lets block inside
				for _, nestedBlock := range block.Body.Blocks {
					if nestedBlock.Type == "lets" {
						for _, attr := range nestedBlock.Body.Attributes {
							if attr.Name == attrName {
								location = createAttrLocation(fname, attr)
								return nil
							}
						}
					}
				}
			}
		}
		return nil
	})

	return location, nil
}

// findTerramateDefinition finds definitions for terramate namespace references
func (s *Server) findTerramateDefinition(fname string, traversal hcl.Traversal) (*lsp.Location, error) {
	if len(traversal) < 2 {
		return nil, nil
	}

	workspace, err := s.findWorkspaceForDir(filepath.Dir(fname))
	if err != nil {
		return nil, err
	}

	// Check if this is a terramate.stack.* reference
	secondPart := traversal[1]
	if attr, ok := secondPart.(hcl.TraverseAttr); ok && attr.Name == "stack" {
		// This is accessing terramate.stack.something
		if len(traversal) >= 3 {
			// terramate.stack.name, terramate.stack.id, etc.
			// Navigate to the stack block attribute
			stackAttrTraverse, ok := traversal[2].(hcl.TraverseAttr)
			if !ok {
				return nil, nil
			}
			stackAttr := stackAttrTraverse.Name
			return s.findStackAttributeDefinition(fname, stackAttr)
		}
	}

	// Check if this is a terramate.run.env.* reference
	if envVarName := extractEnvVarName(traversal); envVarName != "" {
		// Environment variables can be defined at stack-level or project-wide
		// Search hierarchically from current directory up to workspace root
		// Note: findDefinitions handles returning all definitions for go-to-definition
		dir := filepath.Dir(fname)
		for {
			location, found, err := s.searchEnvInDir(dir, envVarName)
			if err != nil {
				return nil, err
			}
			if found {
				return location, nil
			}

			// Move to parent directory
			parent := filepath.Dir(dir)
			if parent == dir || !strings.HasPrefix(parent, workspace) {
				// Reached root or left workspace
				break
			}
			dir = parent
		}
	}

	// Other terramate.* references (terramate.path, terramate.root, etc.)
	// are built-in and don't have user-defined locations
	return nil, nil
}

// findLabelDefinitionsAtPosition checks if cursor is on a label in a block definition
// and returns all definitions with the same label path
func (s *Server) findLabelDefinitionsAtPosition(body *hclsyntax.Body, targetPos hcl.Pos, fname string) []lsp.Location {
	var labelPath []string
	var found bool

	// Check all blocks to see if cursor is on a label
	_ = hclsyntax.VisitAll(body, func(node hclsyntax.Node) hcl.Diagnostics {
		if found {
			return nil // Already found, stop searching
		}

		if block, ok := node.(*hclsyntax.Block); ok {
			// Only handle globals blocks with labels
			if block.Type != "globals" || len(block.Labels) == 0 {
				return nil
			}

			// Check each label to see if cursor is on it
			for i, labelRange := range block.LabelRanges {
				if posInRange(targetPos, labelRange) {
					// Cursor is on this label - build path up to and including this label
					labelPath = block.Labels[:i+1]
					found = true
					s.log.Debug().
						Strs("labelPath", labelPath).
						Int("labelIndex", i).
						Msg("findLabelDefinitionsAtPosition: cursor on label in block definition")
					return nil
				}
			}
		}
		return nil
	})

	if !found || len(labelPath) == 0 {
		return nil
	}

	// Find all blocks with this label path across the hierarchy
	locations := s.findAllLabeledBlockDefinitions(fname, labelPath)
	return locations
}

// findStackAttributeDefinition finds the definition of a stack attribute
func (s *Server) findStackAttributeDefinition(fname string, attrName string) (*lsp.Location, error) {
	// Search for stack blocks in the current directory and parents
	dir := filepath.Dir(fname)

	workspace, err := s.findWorkspaceForDir(dir)
	if err != nil {
		return nil, err
	}

	for {
		// Look for stack definitions in .tm and .tm.hcl files in this directory
		location, found, err := s.searchStackInDir(dir, attrName)
		if err != nil {
			return nil, err
		}
		if found {
			return location, nil
		}

		// Move to parent directory
		parent := filepath.Dir(dir)
		if parent == dir || !strings.HasPrefix(parent, workspace) {
			// Reached root or left workspace
			break
		}
		dir = parent
	}

	return nil, nil
}

// findStringLiteralDefinition checks if the position is in a string literal and handles:
// - import.source attributes (navigate to the imported file)
// - stack.after/before/wants/wanted_by arrays (navigate to referenced stacks)
func (s *Server) findStringLiteralDefinition(body *hclsyntax.Body, targetPos hcl.Pos, fname string) *lsp.Location {
	var location *lsp.Location

	_ = hclsyntax.VisitAll(body, func(node hclsyntax.Node) hcl.Diagnostics {
		// If we already found a location, stop visiting
		if location != nil {
			return nil
		}

		if block, ok := node.(*hclsyntax.Block); ok {
			// Handle import blocks
			if block.Type == "import" {
				if sourceAttr, ok := block.Body.Attributes["source"]; ok {
					// Check if cursor is anywhere on the source attribute (more lenient)
					if posInRange(targetPos, sourceAttr.Expr.Range()) || posInRange(targetPos, sourceAttr.NameRange) {
						// Try to extract the source path
						var sourcePath string

						// Handle template expressions (quoted strings)
						if litExpr, ok := sourceAttr.Expr.(*hclsyntax.TemplateExpr); ok {
							if len(litExpr.Parts) == 1 {
								if lit, ok := litExpr.Parts[0].(*hclsyntax.LiteralValueExpr); ok {
									sourcePath = lit.Val.AsString()
								}
							}
						}

						// Handle direct literal (unquoted, rare but possible)
						if litExpr, ok := sourceAttr.Expr.(*hclsyntax.LiteralValueExpr); ok {
							sourcePath = litExpr.Val.AsString()
						}

						if sourcePath != "" {
							loc := s.resolveImportPath(fname, sourcePath)
							if loc != nil {
								location = loc
								return nil // Return early when import path is found!
							}
						}
					}
				}
			}

			// Handle stack blocks with dependencies
			if block.Type == "stack" {
				for _, attrName := range []string{"after", "before", "wants", "wanted_by"} {
					if attr, ok := block.Body.Attributes[attrName]; ok {
						location = s.findStackRefInArray(attr.Expr, targetPos, fname)
						if location != nil {
							return nil
						}
					}
				}
			}
		}
		return nil
	})

	return location
}

// findStackRefInArray finds stack references in array expressions
func (s *Server) findStackRefInArray(expr hcl.Expression, targetPos hcl.Pos, fname string) *lsp.Location {
	tupleExpr, ok := expr.(*hclsyntax.TupleConsExpr)
	if !ok {
		return nil
	}

	for _, elem := range tupleExpr.Exprs {
		// Check if cursor is in this array element (more lenient)
		if posInRange(targetPos, elem.Range()) {
			var stackPath string

			// Handle template expressions (quoted strings)
			if tmplExpr, ok := elem.(*hclsyntax.TemplateExpr); ok {
				if len(tmplExpr.Parts) == 1 {
					if lit, ok := tmplExpr.Parts[0].(*hclsyntax.LiteralValueExpr); ok {
						stackPath = lit.Val.AsString()
					}
				}
			}

			// Handle direct literal
			if litExpr, ok := elem.(*hclsyntax.LiteralValueExpr); ok {
				stackPath = litExpr.Val.AsString()
			}

			if stackPath != "" {
				return s.findStackByPath(fname, stackPath)
			}
		}
	}

	return nil
}

// resolveImportPath resolves an import source path to a file location.
func (s *Server) resolveImportPath(currentFile string, sourcePath string) *lsp.Location {
	var absPath string

	workspace, err := s.findWorkspaceForDir(filepath.Dir(currentFile))
	if err != nil {
		s.log.Debug().
			Str("currentFile", currentFile).
			Msg("file not in any current workspace")
		return nil
	}

	if filepath.IsAbs(sourcePath) || strings.HasPrefix(sourcePath, "/") {
		absPath = filepath.Join(workspace, strings.TrimPrefix(sourcePath, "/"))
	} else {
		currentDir := filepath.Dir(currentFile)
		absPath = filepath.Join(currentDir, sourcePath)
	}

	if strings.Contains(absPath, "*") {
		return nil
	}

	resolvedPath, err := filepath.Abs(absPath)
	if err != nil {
		s.log.Debug().Err(err).Str("sourcePath", sourcePath).Msg("failed to resolve import path to absolute path")
		return nil
	}

	if workspace != "" && !strings.HasPrefix(resolvedPath, workspace) {
		s.log.Debug().
			Str("sourcePath", sourcePath).
			Str("resolvedPath", resolvedPath).
			Str("workspace", workspace).
			Msg("import path resolves outside workspace - possible directory traversal attempt")
		return nil
	}

	if _, err := os.Stat(resolvedPath); err != nil {
		s.log.Debug().Err(err).Str("path", resolvedPath).Msg("import path does not exist")
		return nil
	}

	return &lsp.Location{
		URI: lsp.URI(uri.File(filepath.ToSlash(resolvedPath))),
		Range: lsp.Range{
			Start: lsp.Position{Line: 0, Character: 0},
			End:   lsp.Position{Line: 0, Character: 0},
		},
	}
}

// findStackByPath finds a stack definition by its path.
func (s *Server) findStackByPath(fname, stackPath string) *lsp.Location {
	var searchDir string

	workspace, err := s.findWorkspaceForDir(filepath.Dir(fname))
	if err != nil {
		s.log.Debug().
			Str("stackPath", stackPath).
			Msg("stack not in any current workspace")
		return nil
	}

	if filepath.IsAbs(stackPath) || strings.HasPrefix(stackPath, "/") {
		searchDir = filepath.Join(workspace, strings.TrimPrefix(stackPath, "/"))
	} else {
		searchDir = filepath.Join(workspace, stackPath)
	}

	searchDir = filepath.Clean(searchDir)
	resolvedDir, err := filepath.Abs(searchDir)
	if err != nil {
		s.log.Debug().Err(err).Str("stackPath", stackPath).Msg("failed to resolve stack path to absolute path")
		return nil
	}

	if workspace != "" && !strings.HasPrefix(resolvedDir, workspace) {
		s.log.Debug().
			Str("stackPath", stackPath).
			Str("resolvedDir", resolvedDir).
			Str("workspace", workspace).
			Msg("stack path resolves outside workspace - possible directory traversal attempt")
		return nil
	}

	searchDir = resolvedDir
	entries, err := os.ReadDir(searchDir)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		if !isTerramateFile(filename) {
			continue
		}

		fullPath := filepath.Join(searchDir, filename)
		location, found := s.findStackBlockInFile(fullPath)
		if found {
			return location
		}
	}

	return nil
}

// findStackBlockInFile finds the stack block in a file
func (s *Server) findStackBlockInFile(fname string) (*lsp.Location, bool) {
	syntaxBody, err := parseHCLFile(fname)
	if err != nil || syntaxBody == nil {
		return nil, false
	}

	var location *lsp.Location
	var found bool

	_ = hclsyntax.VisitAll(syntaxBody, func(node hclsyntax.Node) hcl.Diagnostics {
		if block, ok := node.(*hclsyntax.Block); ok {
			if block.Type == "stack" {
				location = &lsp.Location{
					URI:   lsp.URI(uri.File(filepath.ToSlash(fname))),
					Range: hclRangeToLSP(block.TypeRange),
				}
				found = true
				return nil
			}
		}
		return nil
	})

	return location, found
}
