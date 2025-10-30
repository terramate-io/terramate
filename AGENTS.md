# AGENTS.md

This file provides guidance for AI coding agents working on the Terramate project.

---

## Project Overview

**Terramate** is an orchestration, code generation, and change management tool for Infrastructure as Code (IaC), with first-class support for Terraform, OpenTofu, and Terragrunt.

**Repository Structure**:
- `/cmd/` - Main binaries (terramate CLI, terramate-ls language server)
- `/ls/` - Language Server Protocol implementation
- `/hcl/` - HCL parsing and evaluation
- `/config/` - Configuration management
- `/stack/` - Stack orchestration
- `/cloud/` - Terramate Cloud integration
- `/e2etests/` - End-to-end tests
- `/test/` - Test utilities

**Language**: Go 1.24+
**License**: MPL-2.0

---

## Setup Commands

### Prerequisites

```bash
# Install all dependencies using the ASDF package manager
asdf install

# Check versions
go version  # Should be 1.24+
make --version
```

### Build

```bash
# Build all binaries
make build

# Output:
# - bin/terramate (CLI)
# - bin/terramate-ls (Language Server)
# - bin/helper (test helper)
```

### Development

```bash
# Run all tests
make test

# Run specific package tests
go test ./ls/...
go test ./hcl/...

# Run with race detector
go test -race ./ls/...

# Format code
make fmt

# Install linting
make lint/install

# Run linting
make lint/all
```

---

## Code Style

### Go Standards

- **Follow standard Go conventions**: Use `gofmt`, pass `go vet`
- **Error handling**: Always check errors, use `errors.E()` wrapper for context
- **Naming**: Use descriptive names, avoid abbreviations except common ones (e.g., `ctx`, `err`)
- **Comments**: Public functions must have doc comments starting with function name

### Project-Specific Patterns

**Error handling**:
```go
// Use errors.E() for wrapping
return nil, errors.E(err, "description of what failed")

// Use errors.L() for collecting multiple errors
errs := errors.L()
errs.Append(err1)
errs.Append(err2)
return errs.AsError()
```

**Logging**:
```go
// Use zerolog for structured logging
log.Debug().Str("key", value).Msg("description")
log.Info().Int("count", n).Msg("operation complete")
log.Error().Err(err).Msg("operation failed")
```

**Testing**:
```go
// Use test helpers from test/ package
s := sandbox.New(t)
s.BuildTree(layout)

// Use assert package, not testing.T directly
assert.NoError(t, err)
assert.EqualStrings(t, want, got)
assert.IsTrue(t, condition, "message")
```

---

## Language Server Development (`/ls/` directory)

### Key Files

**Core Implementation**:
- `ls.go` - Main server, handler registration, LSP capabilities
- `definition.go` - Go-to definition implementation
- `references.go` - Find all references
- `rename.go` - Rename symbol
- `imports.go` - Import resolution
- `label_rename.go` - Label renaming
- `util.go` - Shared utilities
- `hcl_helpers.go` - HCL parsing helpers

**Test Files**:
- `definition_test.go` - Definition tests 
- `references_test.go` - Reference tests
- `rename_test.go` - Rename tests
- `label_rename_test.go` - Label rename tests 
- `ls_test.go` - Core server tests
- `commands_test.go` - Command tests 
- `position_test.go` - Position handling tests
- `util_test.go` - Utility tests
- `benchmark_test.go` - Performance benchmarks 
- `document_lifecycle_test.go` - Document sync tests

### Important Patterns

**LSP Handler Structure**:
```go
func (s *Server) handleFeature(
    ctx context.Context,
    reply jsonrpc2.Replier,
    r jsonrpc2.Request,
    log zerolog.Logger,
) error {
    // 1. Unmarshal params
    // 2. Process request
    // 3. Return reply
    return reply(ctx, result, nil)
}
```

**Definition Search Pattern**:
```go
// 1. Search current directory
// 2. Search parent directories (recursively up to workspace root)
// 3. Search imported files (follow import chains)
// 4. Return first match (child overrides parent)
```

**Testing Pattern**:
```go
// Use sandbox for file system tests
s := sandbox.New(t)
s.BuildTree([]string{
    `f:globals.tm:globals { var = "value" }`,
    `f:stack.tm:stack { name = global.var }`,
})

// Create test server
srv := newTestServer(t, s.RootDir())

// Test functionality
location, err := srv.findDefinition(...)
assert.NoError(t, err)
```

---

## Terramate-Specific Concepts

### Variable Namespaces

**Understand these Terramate namespaces**:

1. **`global.*`** - Global variables (can be defined in parent directories, imported files, labeled blocks)
   - Simple: `globals { my_var = "value" }` → `global.my_var`
   - Labeled: `globals "a" "b" { c = "x" }` → `global.a.b.c`
   - Nested: `globals { a = { b = { c = "x" } } }` → `global.a.b.c`

2. **`let.*`** - Let variables (scoped to generate blocks)
   - `generate_hcl { lets { x = "y" } }` → `let.x`

3. **`terramate.stack.*`** - Stack metadata (built-in)
   - `terramate.stack.name`, `terramate.stack.id`, etc.

4. **`env.*`** - Environment variables (defined in `terramate.config.run.env`)

5. **`stack.*`** - NOT VALID (use `terramate.stack.*` instead)

### Import Resolution

**Critical**: Globals can be defined in imported files:

```hcl
# File A
globals { project_id = "123" }

# File B
import { source = "/file_a.tm" }
globals { x = global.project_id }  # Must resolve through import!
```

**Implementation must**:
- Parse import statements
- Recursively follow import chains
- Search imported files for definitions
- Handle circular imports (visited tracking)

### Hierarchical Overrides

**Child overrides parent**:

```hcl
# Parent: /globals.tm
globals { env = "default" }

# Child: /stacks/prod/globals.tm
globals { env = "production" }  # This wins!

# Search order: Current dir → Parent dirs → Imports
```

---

## Testing Instructions

### Running Tests

```bash
# All tests
make test

# Specific package
go test ./ls/...

# Verbose
go test -v ./ls/...

# With race detector (important!)
go test -race ./ls/...

# Specific test
go test ./ls/... -run TestFindDefinition

# With coverage
go test -cover ./ls/...
```

### Test Requirements

**Before committing**:
- ✅ All tests must pass: `go test ./ls/...`
- ✅ Race detector clean: `go test -race ./ls/...`
- ✅ Linting clean: `golangci-lint run ./ls/...`
- ✅ Formatting correct: `gofmt -l ls/*.go` (should return empty)

**When adding features**:
- ✅ Add tests in `*_test.go` files
- ✅ Use table-driven tests for multiple scenarios
- ✅ Test edge cases (empty files, malformed HCL, circular imports)
- ✅ Test real-world scenarios (like iac-gcloud examples)

### Test File Patterns

Use sandbox for LSP tests:
```go
s := sandbox.New(t)
s.BuildTree([]string{
    `f:path/to/file.tm:content`,
    `s:stack/path`,  // Create stack
})
```

---

## Language Server Implementation Guidelines

### Key Design Principles

1. **Performance**: Only parse files as needed, cache where possible
2. **Robustness**: Handle errors gracefully (return nil, don't crash)
3. **Completeness**: Search current directory, parents, AND imports
4. **Correctness**: Match Terramate's runtime behavior exactly

### Critical Implementation Details

**Import resolution** (recursive with circular protection):
```go
// Search strategy (in imports.go):
// 1. Search current directory
// 2. Extract imports from all .tm files in directory
// 3. Search each imported file
// 4. Extract imports from imported file (nested imports)
// 5. Recursively search those (can be N levels deep!)
// 6. Move to parent directory and repeat
// 7. Track visited files to prevent circular import loops

// Real-world example (iac-gcloud):
// File A → imports default.tm
// default.tm → imports 13 other files
// One of those defines the global
// System finds it through the entire chain!
```

**Label vs Nested Object detection**:
```go
// Both create global.a.b:
globals "a" "b" { ... }           // Labeled block
globals { a = { b = { ... } } }   // Nested object

// Detection (in references.go):
// - Check if labeled block exists: hasLabeledBlockWithPath()
// - Search current dir, parents, AND imports
// - Used for rename logic to differentiate label from object key
```

**Import resolution** (recursive):
```go
// Must follow import chains:
// File A imports B, B imports C, C defines global
// Search order: A → A's imports → B → B's imports → C
```

### Performance Considerations

**Expensive operations** (cache/optimize):
- File parsing (use visited maps)
- Directory scanning (early exit on match)
- Import resolution (visited tracking for circular imports)

**Cheap operations**:
- AST traversal (after parsing)
- Range checks
- String comparisons

---

## Common Pitfalls

### 1. HCL Coordinate System

**HCL uses 1-indexed** positions, **LSP uses 0-indexed**:

```go
// Converting HCL to LSP
lspLine = hclLine - 1
lspChar = hclChar - 1

// TraverseAttr.SrcRange includes the preceding dot!
// For ".attr", SrcRange.Start points to the dot, not 'a'
// Add 1 to skip the dot when needed
```

### 2. Import Paths

**Absolute vs Relative**:
```go
// Absolute (project-relative): starts with /
source = "/modules/shared/globals.tm"
// Resolve: workspace + source

// Relative: no leading /
source = "../shared/globals.tm"  
// Resolve: currentDir + source
```

### 3. Labeled Globals Matching

```go
// Path ["a", "b", "c"] can match:
globals "a" "b" "c" { ... }           // Exact match
globals "a" "b" "c" "d" { ... }       // Prefix match
globals "a" "b" { c = "..." }         // Labels + attribute

// Use matchesLabeledGlobal() carefully!
```

### 4. Test Stability

**Avoid**:
- Hard-coded line numbers (use relative positions)
- Assuming file order (files may be parsed in any order)
- Timing dependencies (use proper synchronization)

**Do**:
- Use `t.Parallel()` for test concurrency
- Clean up temporary files
- Use `sandbox.New(t)` for isolated test environments

---

## Security Considerations

### Path Traversal Prevention

**Always validate**:
- Workspace boundaries (use `strings.HasPrefix(path, s.workspace)`)
- No escaping workspace root (`..` attacks)
- Import paths don't escape workspace

```go
// Good
if !strings.HasPrefix(resolvedPath, s.workspace) {
    return nil, errors.E("path outside workspace")
}

// Bad  
// Blindly joining paths without validation
```

### Race Conditions

**Use mutexes for**:
- Shared caches
- Concurrent map access
- File system operations from multiple goroutines

**Test with**: `go test -race`

---

## PR Instructions

### Before Submitting

1. **Run full test suite**: `make test`
2. **Run race detector**: `go test -race ./ls/...`
3. **Format code**: `make fmt`
4. **Check linting**: `make lint/all`
5. **Update documentation**: If adding features, update `ls/README.md`

### PR Title Format

```
feat(ls): add cursor-aware path navigation
fix(ls): correct import resolution for circular imports
docs(ls): update README with label renaming examples
test(ls): add tests for nested object navigation
```

### PR Description Should Include

- **What**: Feature/fix description
- **Why**: Problem being solved
- **How**: Implementation approach
- **Testing**: How it was tested
- **Breaking changes**: If any

### Review Checklist

- [ ] Tests pass (including race detector)
- [ ] No new linter errors
- [ ] Code formatted (gofmt)
- [ ] Documentation updated
- [ ] No commented-out code
- [ ] Error messages are helpful
- [ ] Edge cases tested

---

## Known Complex Areas

### Import Resolution (`ls/imports.go`)

**Complexity**: High  
**Why**: Recursive, circular import detection, path resolution

**Key functions**:
- `findGlobalWithImports()` - Entry point
- `searchWithImports()` - Searches dir + imports
- `searchImportedFiles()` - Recursive import following
- `collectImportsFromDir()` - Extracts imports from files

**Testing**: Use nested import chains, circular imports, relative paths

### Label Renaming (`ls/label_rename.go`)

**Complexity**: Very High  
**Why**: Must distinguish labels from nested objects, update both definitions and references

**Key challenge**: `global.a.b.c` could be:
- `globals "a" "b" { c = "..." }` (labels)
- `globals { a = { b = { c = "..." } } }` (nested)

**Must check actual definition** to determine which!

### Cursor-Aware Navigation

**Complexity**: High  
**Why**: Must detect precise cursor position, truncate paths correctly

**Key function**: `truncateTraversalAtCursor()` in `util.go`

**Testing**: Click on different parts of `global.a.b.c.d.e.f`

---

## Debugging Tips

### Enable Debug Logging

```bash
# Start language server with debug output
./bin/terramate-ls --log-level debug --log-fmt console 2> ls-debug.log

# Watch logs
tail -f ls-debug.log
```

### VSCode Output Panel

When testing in VSCode:
1. View → Output
2. Select "Terramate Language Server" from dropdown
3. See real-time LSP communication

### Common Issues

**Go-to definition not working**:
- Check workspace root is correct
- Verify import paths resolve correctly
- Check if cursor truncation is interfering

**Rename not working**:
- Check if `isLabelComponent` detection is correct
- Verify `findAllReferences` returns correct ranges
- Check workspace edit is being created

**Tests failing**:
- Run single test: `go test ./ls/... -run TestName -v`
- Check for race conditions: `go test -race`
- Verify sandbox setup is correct

---

## References

**Terramate Documentation**:
- https://terramate.io/docs/cli/
- https://terramate.io/docs/cli/reference/variables/globals
- https://terramate.io/docs/cli/reference/variables/map

**LSP Specification**:
- https://microsoft.github.io/language-server-protocol/

**HCL Parser**:
- https://github.com/hashicorp/hcl (v2)

**Testing**:
- Use `test/sandbox` for file system isolation
- Use `test/hclutils` for HCL test helpers

---
