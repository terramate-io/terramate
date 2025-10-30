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
)

func (s *Server) handlePrepareRename(
	ctx context.Context,
	reply jsonrpc2.Replier,
	r jsonrpc2.Request,
	log zerolog.Logger,
) error {
	var params lsp.PrepareRenameParams
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
		Msg("handling prepare rename")

	// Get file content (from cache if available, or disk)
	content, err := s.getDocumentContent(fname)
	if err != nil {
		log.Error().Err(err).Msg("failed to get file content")
		return reply(ctx, nil, nil)
	}

	// Check if the position is on a renameable symbol
	renameInfo := s.canRename(fname, content, line, character)
	if renameInfo == nil {
		// Not a renameable symbol
		return reply(ctx, nil, nil)
	}

	// Return the range of the symbol that will be renamed
	return reply(ctx, renameInfo, nil)
}

func (s *Server) handleRename(
	ctx context.Context,
	reply jsonrpc2.Replier,
	r jsonrpc2.Request,
	log zerolog.Logger,
) error {
	var params lsp.RenameParams
	if err := json.Unmarshal(r.Params(), &params); err != nil {
		log.Error().Err(err).Msg("failed to unmarshal params")
		return jsonrpc2.ErrParse
	}

	fname := params.TextDocument.URI.Filename()
	line := params.Position.Line
	character := params.Position.Character
	newName := params.NewName

	log.Debug().
		Str("file", fname).
		Uint32("line", line).
		Uint32("character", character).
		Str("newName", newName).
		Msg("handling rename")

	// Get file content (from cache if available, or disk)
	content, err := s.getDocumentContent(fname)
	if err != nil {
		log.Error().Err(err).Msg("failed to get file content")
		return reply(ctx, nil, nil)
	}

	// Validate the new name
	if !isValidIdentifier(newName) {
		log.Error().Str("newName", newName).Msg("invalid identifier name")
		return reply(ctx, nil, nil)
	}

	// Find all references and create edits
	workspaceEdit, err := s.createRenameEdits(ctx, fname, content, line, character, newName)
	if err != nil {
		log.Error().Err(err).Msg("failed to create rename edits")
		return reply(ctx, nil, nil)
	}

	if workspaceEdit == nil {
		return reply(ctx, nil, nil)
	}

	return reply(ctx, workspaceEdit, nil)
}

// canRename checks if the symbol at the position can be renamed
func (s *Server) canRename(fname string, content []byte, line, character uint32) *lsp.Range {
	s.log.Debug().
		Str("file", fname).
		Uint32("line", line).
		Uint32("character", character).
		Msg("canRename: checking if symbol can be renamed")

	syntaxBody, err := parseHCLContent(content, fname)
	if err != nil {
		s.log.Debug().Err(err).Msg("canRename: failed to parse file")
		return nil
	}
	if syntaxBody == nil {
		s.log.Debug().Msg("canRename: file body is not syntax body")
		return nil
	}

	targetPos := hcl.Pos{
		Line:   int(line) + 1,
		Column: int(character) + 1,
		Byte:   posToByteOffset(content, int(line), int(character)),
	}

	// Find if this is a renameable symbol
	symbolInfo := s.findSymbolAtPosition(syntaxBody, targetPos)
	if symbolInfo == nil {
		s.log.Debug().Msg("canRename: no symbol found at position")
		return nil
	}

	s.log.Debug().
		Str("namespace", symbolInfo.namespace).
		Str("attributeName", symbolInfo.attributeName).
		Str("fullPath", symbolInfo.fullPath).
		Strs("pathComponents", symbolInfo.pathComponents).
		Int("componentIndex", symbolInfo.componentIndex).
		Bool("isLabelComponent", symbolInfo.isLabelComponent).
		Msg("canRename: found symbol")

	// Check if this is potentially a label component (not final attribute)
	if symbolInfo.isLabelComponent && symbolInfo.namespace == "global" {
		// Verify it's actually a labeled block (not nested object)
		pathUpToComponent := symbolInfo.pathComponents[:symbolInfo.componentIndex+1]

		s.log.Debug().
			Strs("pathUpToComponent", pathUpToComponent).
			Msg("canRename: checking if this is a labeled block")

		isActuallyLabel := s.hasLabeledBlockWithPath(fname, pathUpToComponent)

		s.log.Debug().
			Bool("isActuallyLabel", isActuallyLabel).
			Msg("canRename: labeled block check result")

		if isActuallyLabel {
			s.log.Debug().
				Str("component", symbolInfo.pathComponents[symbolInfo.componentIndex]).
				Int("index", symbolInfo.componentIndex).
				Msg("canRename: detected ACTUAL label component - can rename")

			// Return the range for this label component
			var labelRange *lsp.Range
			_ = hclsyntax.VisitAll(syntaxBody, func(node hclsyntax.Node) hcl.Diagnostics {
				if expr, ok := node.(hclsyntax.Expression); ok {
					if scopeExpr, ok := expr.(*hclsyntax.ScopeTraversalExpr); ok {
						if posInRange(targetPos, scopeExpr.Range()) {
							if symbolInfo.componentIndex+1 < len(scopeExpr.Traversal) {
								if attr, ok := scopeExpr.Traversal[symbolInfo.componentIndex+1].(hcl.TraverseAttr); ok {
									if posInRange(targetPos, attr.SrcRange) {
										r := hclTraverseAttrToLSP(attr.SrcRange)
										labelRange = &r
									}
								}
							}
						}
					}
				}
				return nil
			})

			return labelRange
		}

		// Not actually a label (it's a nested object key) - treat as regular attribute
		s.log.Debug().Msg("potential label is actually nested object - treating as attribute")
	}

	// Only allow renaming user-defined variables: global, let, and terramate.run.env
	// Note: terramate.run.env.* refers to environment variables defined in terramate.config.run.env blocks
	// All other terramate.* metadata (including terramate.stack.*) is protected and cannot be renamed
	// Stack attributes in stack {} blocks are also fixed metadata and cannot be renamed
	switch symbolInfo.namespace {
	case "global", "let", "terramate.run.env":
		// These can be renamed
		s.log.Debug().
			Str("namespace", symbolInfo.namespace).
			Msg("canRename: namespace is renameable")
	default:
		// Other terramate.* namespaces (metadata) cannot be renamed
		s.log.Debug().
			Str("namespace", symbolInfo.namespace).
			Msg("canRename: namespace cannot be renamed (protected metadata)")
		return nil
	}

	// Check if we're on an attribute definition or reference
	var symbolRange *lsp.Range
	_ = hclsyntax.VisitAll(syntaxBody, func(node hclsyntax.Node) hcl.Diagnostics {
		if block, ok := node.(*hclsyntax.Block); ok {
			// Check in globals blocks
			if block.Type == "globals" {
				for _, attr := range block.Body.Attributes {
					if posInRange(targetPos, attr.NameRange) && attr.Name == symbolInfo.attributeName {
						r := hclNameRangeToLSP(attr.NameRange)
						symbolRange = &r
						return nil
					}
				}

				// Check map blocks in globals
				for _, nested := range block.Body.Blocks {
					if nested.Type == "map" {
						// Check if cursor is on the map block label itself
						if len(nested.Labels) > 0 && len(nested.LabelRanges) > 0 {
							if posInRange(targetPos, nested.LabelRanges[0]) && nested.Labels[0] == symbolInfo.attributeName {
								symbolRange = &lsp.Range{
									Start: lsp.Position{
										Line:      uint32(nested.LabelRanges[0].Start.Line - 1),
										Character: uint32(nested.LabelRanges[0].Start.Column - 1),
									},
									End: lsp.Position{
										Line:      uint32(nested.LabelRanges[0].End.Line - 1),
										Character: uint32(nested.LabelRanges[0].End.Column - 1),
									},
								}
								return nil
							}
						}

						// Check attributes inside the map block
						for _, attr := range nested.Body.Attributes {
							if posInRange(targetPos, attr.NameRange) && attr.Name == symbolInfo.attributeName {
								symbolRange = &lsp.Range{
									Start: lsp.Position{
										Line:      uint32(attr.NameRange.Start.Line - 1),
										Character: uint32(attr.NameRange.Start.Column - 1),
									},
									End: lsp.Position{
										Line:      uint32(attr.NameRange.End.Line - 1),
										Character: uint32(attr.NameRange.End.Column - 1),
									},
								}
								return nil
							}
						}
					}
				}
			}

			// Check in stack blocks
			if block.Type == "stack" {
				for _, attr := range block.Body.Attributes {
					if posInRange(targetPos, attr.NameRange) && attr.Name == symbolInfo.attributeName {
						symbolRange = &lsp.Range{
							Start: lsp.Position{
								Line:      uint32(attr.NameRange.Start.Line - 1),
								Character: uint32(attr.NameRange.Start.Column - 1),
							},
							End: lsp.Position{
								Line:      uint32(attr.NameRange.End.Line - 1),
								Character: uint32(attr.NameRange.End.Column - 1),
							},
						}
						return nil
					}
				}
			}

			// Check in lets blocks
			if block.Type == "generate_hcl" || block.Type == "generate_file" {
				for _, nested := range block.Body.Blocks {
					if nested.Type == "lets" {
						for _, attr := range nested.Body.Attributes {
							if posInRange(targetPos, attr.NameRange) && attr.Name == symbolInfo.attributeName {
								symbolRange = &lsp.Range{
									Start: lsp.Position{
										Line:      uint32(attr.NameRange.Start.Line - 1),
										Character: uint32(attr.NameRange.Start.Column - 1),
									},
									End: lsp.Position{
										Line:      uint32(attr.NameRange.End.Line - 1),
										Character: uint32(attr.NameRange.End.Column - 1),
									},
								}
								return nil
							}
						}
					}
				}
			}

			// Check in terramate.config.run.env blocks
			if block.Type == "terramate" {
				for _, configBlock := range block.Body.Blocks {
					if configBlock.Type == "config" {
						for _, runBlock := range configBlock.Body.Blocks {
							if runBlock.Type == "run" {
								for _, envBlock := range runBlock.Body.Blocks {
									if envBlock.Type == "env" {
										// Check attributes in env block
										for _, attr := range envBlock.Body.Attributes {
											if posInRange(targetPos, attr.NameRange) && attr.Name == symbolInfo.attributeName {
												symbolRange = &lsp.Range{
													Start: lsp.Position{
														Line:      uint32(attr.NameRange.Start.Line - 1),
														Character: uint32(attr.NameRange.Start.Column - 1),
													},
													End: lsp.Position{
														Line:      uint32(attr.NameRange.End.Line - 1),
														Character: uint32(attr.NameRange.End.Column - 1),
													},
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

		// If not on definition, check if on a reference
		if expr, ok := node.(hclsyntax.Expression); ok {
			if scopeExpr, ok := expr.(*hclsyntax.ScopeTraversalExpr); ok {
				if posInRange(targetPos, scopeExpr.Range()) {
					if len(scopeExpr.Traversal) >= 2 {
						// Find which part of the traversal the cursor is on
						// For global.a.b.c.region, cursor must be on "region" (the last part)
						// For terramate.stack.name, cursor can only be on "name" (not "stack")

						rootName := scopeExpr.Traversal.RootName()
						var attrIdx int

						// Handle terramate.stack.* references - special case
						// For terramate.stack.name, the attribute is at traversal[2], not traversal[1]
						// Handle terramate.run.env.* references - special case
						// For terramate.run.env.MY_VAR, the attribute is at traversal[3]
						if rootName == "terramate" && len(scopeExpr.Traversal) >= 3 {
							if attr, ok := scopeExpr.Traversal[1].(hcl.TraverseAttr); ok && attr.Name == "stack" {
								// For terramate.stack.name, we want traversal[2] (the attribute name)
								attrIdx = 2
							} else if isEnvVarTraversal(scopeExpr.Traversal) {
								// For terramate.run.env.MY_VAR, we want traversal[3] (the env variable name)
								attrIdx = 3
							} else {
								// Not a terramate.stack.* or terramate.run.env.* reference, use last index
								attrIdx = len(scopeExpr.Traversal) - 1
							}
						} else {
							// For other references (global, let, etc.), use the last attribute
							attrIdx = len(scopeExpr.Traversal) - 1
						}

						if attrIdx >= len(scopeExpr.Traversal) || attrIdx < 1 {
							return nil
						}

						attr, ok := scopeExpr.Traversal[attrIdx].(hcl.TraverseAttr)
						if !ok {
							return nil
						}

						// Only allow rename if cursor is on the correct part
						if !posInRange(targetPos, attr.SrcRange) {
							return nil
						}

						// TraverseAttr.SrcRange includes the preceding dot
						// For ".region" HCL gives Column=16 (1-indexed, the dot)
						// To skip dot and point to "region": LSP Char = HCL Col (no -1!)
						// For end: normal conversion with -1
						startLine := attr.SrcRange.Start.Line - 1
						startCol := attr.SrcRange.Start.Column // Keep as-is to skip dot
						endCol := attr.SrcRange.End.Column - 1

						symbolRange = &lsp.Range{
							Start: lsp.Position{
								Line:      uint32(startLine),
								Character: uint32(startCol),
							},
							End: lsp.Position{
								Line:      uint32(startLine),
								Character: uint32(endCol),
							},
						}
					}
				}
			}
		}
		return nil
	})

	return symbolRange
}

// createRenameEdits creates workspace edits for renaming a symbol
func (s *Server) createRenameEdits(ctx context.Context, fname string, content []byte, line, character uint32, newName string) (*lsp.WorkspaceEdit, error) {
	// Validate the new name
	if !isValidIdentifier(newName) {
		return nil, nil
	}

	file, diags := hclsyntax.ParseConfig(content, fname, hcl.InitialPos)
	if diags.HasErrors() {
		return nil, diags
	}

	syntaxBody, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return nil, nil
	}

	targetPos := hcl.Pos{
		Line:   int(line) + 1,
		Column: int(character) + 1,
		Byte:   posToByteOffset(content, int(line), int(character)),
	}

	// Find the symbol to rename
	symbolInfo := s.findSymbolAtPosition(syntaxBody, targetPos)
	if symbolInfo == nil {
		return nil, nil
	}

	// Check if this is a label component rename
	if symbolInfo.isLabelComponent && symbolInfo.namespace == "global" {
		// Verify it's actually a labeled block
		pathUpToComponent := symbolInfo.pathComponents[:symbolInfo.componentIndex+1]
		isActuallyLabel := s.hasLabeledBlockWithPath(fname, pathUpToComponent)

		if isActuallyLabel {
			s.log.Debug().
				Str("component", symbolInfo.pathComponents[symbolInfo.componentIndex]).
				Int("index", symbolInfo.componentIndex).
				Msg("createRenameEdits: performing label component rename")
			return s.renameLabelComponent(ctx, fname, symbolInfo, newName)
		}

		// Not a label - treat as regular attribute rename
		s.log.Debug().Msg("potential label is actually nested object key")
	}

	// Only allow renaming user-defined symbols
	switch symbolInfo.namespace {
	case "global", "let", "terramate.run.env":
		// OK to rename
	default:
		return nil, nil
	}

	// Find all references (including the definition)
	locations, err := s.findAllReferences(ctx, fname, content, line, character, true)
	if err != nil {
		return nil, err
	}

	// For env variables, find ALL definitions across the hierarchy (not just the closest one)
	// This ensures renaming works bidirectionally - renaming any definition or reference
	// updates all definitions at all levels and all references
	if symbolInfo.namespace == "terramate.run.env" {
		allEnvDefs, err := s.findAllEnvInHierarchy(fname, symbolInfo.attributeName)
		if err != nil {
			s.log.Debug().Err(err).Msg("error finding all env definitions")
		}

		// Add all definitions that aren't already in locations
		for _, envDef := range allEnvDefs {
			alreadyIncluded := false
			for _, loc := range locations {
				if loc.URI == envDef.URI &&
					loc.Range.Start.Line == envDef.Range.Start.Line &&
					loc.Range.Start.Character == envDef.Range.Start.Character {
					alreadyIncluded = true
					break
				}
			}
			if !alreadyIncluded {
				locations = append(locations, envDef)
			}
		}
	} else {
		// For other namespaces, find the single definition location
		defLocation := s.findDefinitionForRename(fname, symbolInfo)
		if defLocation != nil {
			// Check if it's not already in locations
			alreadyIncluded := false
			for _, loc := range locations {
				if loc.URI == defLocation.URI &&
					loc.Range.Start.Line == defLocation.Range.Start.Line &&
					loc.Range.Start.Character == defLocation.Range.Start.Character {
					alreadyIncluded = true
					break
				}
			}
			if !alreadyIncluded {
				locations = append([]lsp.Location{*defLocation}, locations...)
			}
		}
	}

	// Create edits for each location
	changes := make(map[lsp.DocumentURI][]lsp.TextEdit)

	for _, location := range locations {
		edit := lsp.TextEdit{
			Range:   location.Range,
			NewText: newName,
		}

		changes[location.URI] = append(changes[location.URI], edit)
	}

	return &lsp.WorkspaceEdit{
		Changes: changes,
	}, nil
}

// findDefinitionForRename finds the definition of a symbol for renaming
// Uses the same search strategy as findAndReturnDefinition in references.go
func (s *Server) findDefinitionForRename(fname string, info *symbolInfo) *lsp.Location {
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

	case "let":
		// Let variables are in the same file
		content, err := os.ReadFile(fname)
		if err != nil {
			return nil
		}
		file, diags := hclsyntax.ParseConfig(content, fname, hcl.InitialPos)
		if diags.HasErrors() {
			return nil
		}
		if body, ok := file.Body.(*hclsyntax.Body); ok {
			return s.findDefinitionLocation(body, info, fname)
		}

	case "terramate.run.env":
		// Environment variables can be defined at stack-level or project-wide
		// Search hierarchically from current directory up to workspace root
		dir := filepath.Dir(fname)
		for {
			location, found, err := s.searchEnvInDir(dir, info.attributeName)
			if err != nil {
				s.log.Debug().Err(err).Str("dir", dir).Msg("error searching env in dir")
			}
			if found {
				return location
			}

			// Move to parent directory
			parent := filepath.Dir(dir)
			if parent == dir || !strings.HasPrefix(parent, s.workspace) {
				// Reached root or left workspace
				break
			}
			dir = parent
		}
	}

	return nil
}

// isValidIdentifier checks if a string is a valid HCL identifier.
// Returns false for:
//   - Empty strings
//   - Identifiers starting with digits
//   - Identifiers containing special characters (except underscore)
//   - HCL reserved keywords
func isValidIdentifier(name string) bool {
	if name == "" {
		return false
	}

	// Check for HCL reserved keywords
	// See: https://github.com/hashicorp/hcl/blob/main/hclsyntax/spec.md#identifiers-and-keywords
	reserved := map[string]bool{
		"for":    true,
		"in":     true,
		"if":     true,
		"else":   true,
		"endif":  true,
		"endfor": true,
		"null":   true,
		"true":   true,
		"false":  true,
	}
	if reserved[name] {
		return false
	}

	// Convert to runes to properly handle multi-byte UTF-8 characters
	runes := []rune(name)

	// Must start with letter or underscore
	if !isLetter(runes[0]) && runes[0] != '_' {
		return false
	}

	// Rest must be letters, digits, or underscores
	for _, r := range runes[1:] {
		if !isLetter(r) && !isDigit(r) && r != '_' {
			return false
		}
	}

	return true
}

func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}
