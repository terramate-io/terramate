// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package genfile implements generate_file code generation.
package genfile

import (
	"fmt"
	"path"
	"sort"

	"github.com/gobwas/glob"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/event"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/hcl/info"
	"github.com/terramate-io/terramate/stdlib"
	tel "github.com/terramate-io/terramate/ui/tui/telemetry"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/lets"
	"github.com/terramate-io/terramate/project"
	"github.com/zclconf/go-cty/cty"
)

const (
	// ErrInvalidContentType indicates the content attribute
	// has an invalid type.
	ErrInvalidContentType errors.Kind = "invalid content type"

	// ErrInvalidConditionType indicates the condition attribute
	// has an invalid type.
	ErrInvalidConditionType errors.Kind = "invalid condition type"

	// ErrContentEval indicates an error when evaluating the content attribute.
	ErrContentEval errors.Kind = "evaluating content"

	// ErrConditionEval indicates an error when evaluating the condition attribute.
	ErrConditionEval errors.Kind = "evaluating condition"

	// ErrInheritEval indicates the failure to evaluate the inherit attribute.
	ErrInheritEval errors.Kind = "evaluating inherit attribute"

	// ErrInvalidInheritType indicates the inherit attribute has an invalid type.
	ErrInvalidInheritType errors.Kind = "invalid inherit type"

	// ErrLabelConflict indicates the two generate_file blocks
	// have the same label.
	ErrLabelConflict errors.Kind = "label conflict detected"
)

const (
	// StackContext is the stack context name.
	StackContext = "stack"

	// RootContext is the root context name.
	RootContext = "root"
)

// File represents generated file from a single generate_file block.
type File struct {
	label     string
	context   string
	origin    info.Range
	body      string
	condition bool
	asserts   []config.Assert
}

// Builtin returns false for generate_file blocks.
func (f File) Builtin() bool { return false }

// Label of the original generate_file block.
func (f File) Label() string {
	return f.label
}

// Body returns the file body.
func (f File) Body() string {
	return f.body
}

// Range returns the range information of the generate_file block.
func (f File) Range() info.Range {
	return f.origin
}

// Condition returns the result of the evaluation of the
// condition attribute for the generated code.
func (f File) Condition() bool {
	return f.condition
}

// Context of the generate_file block.
func (f File) Context() string {
	return f.context
}

// Asserts returns all (if any) of the evaluated assert configs of the
// generate_file block. If [File.Condition] returns false then assert configs
// will always be empty since they are not evaluated at all in that case.
func (f File) Asserts() []config.Assert {
	return f.asserts
}

// Header returns the header of this file.
func (f File) Header() string {
	// For now we don't support headers for arbitrary files
	return ""
}

func (f File) String() string {
	return fmt.Sprintf("generate_file %q (condition %t) (body %q) (origin %q)",
		f.Label(), f.Condition(), f.Body(), f.Range().Path())
}

// Load loads and parses from the file system all generate_file blocks for
// a given stack. It will navigate the file system from the stack dir until
// it reaches rootdir, loading generate_file blocks found on Terramate
// configuration files.
//
// All generate_file blocks must have unique labels, even ones at different
// directories. Any conflicts will be reported as an error.
//
// Metadata and globals for the stack are used on the evaluation of the
// generate_file blocks.
//
// The rootdir MUST be an absolute path.
func Load(
	root *config.Root,
	st *config.Stack,
	parentctx *eval.Context,
	vendorDir project.Path,
	vendorRequests chan<- event.VendorRequest,
) ([]File, error) {
	genFileBlocks, err := loadGenFileBlocks(root, st.Dir)
	if err != nil {
		return nil, errors.E("loading generate_file", err)
	}

	var files []File

	hasBlocksWithRootContext := false
	hasBlocksWithStackContext := false

	for _, genFileBlock := range genFileBlocks {
		if genFileBlock.Context != StackContext {
			hasBlocksWithRootContext = true
			continue
		}
		hasBlocksWithStackContext = true

		name := genFileBlock.Label

		matchedAnyStackFilter := len(genFileBlock.StackFilters) == 0
		for _, cond := range genFileBlock.StackFilters {
			matched := true

			for n, globs := range map[string][]glob.Glob{
				"project path":    cond.ProjectPaths,
				"repository path": cond.RepositoryPaths,
			} {
				if globs != nil && !hcl.MatchAnyGlob(globs, st.Dir.String()) {
					log.Logger.Trace().Msgf("Skipping %q, %s doesn't match any filter in %v", st.Dir, n, globs)
					matched = false
					break
				}
			}

			matchedAnyStackFilter = matchedAnyStackFilter || matched
		}

		if !matchedAnyStackFilter {
			files = append(files, File{
				label:     name,
				origin:    genFileBlock.Range,
				condition: false,
			})
			continue
		}

		vendorTargetDir := project.NewPath(path.Join(
			st.Dir.String(),
			path.Dir(name)))

		evalctx := parentctx.Copy()

		evalctx.SetFunction(stdlib.Name("vendor"), stdlib.VendorFunc(vendorTargetDir, vendorDir, vendorRequests))

		dircfg, _ := root.Lookup(st.Dir)
		file, skip, err := Eval(genFileBlock, dircfg, evalctx)
		if err != nil {
			return nil, err
		}
		if !skip {
			files = append(files, file)
		}
	}

	tel.DefaultRecord.Set(
		tel.BoolFlag("file", hasBlocksWithStackContext, "generate"),
		tel.BoolFlag("file-root", hasBlocksWithRootContext, "generate"),
	)

	sort.Slice(files, func(i, j int) bool {
		return files[i].String() < files[j].String()
	})

	return files, nil
}

// Eval the generate_file block.
func Eval(block hcl.GenFileBlock, cfg *config.Tree, evalctx *eval.Context) (file File, skip bool, err error) {
	name := block.Label
	err = lets.Load(block.Lets, evalctx)
	if err != nil {
		return File{}, false, err
	}

	condition := true
	if block.Condition != nil {
		value, err := evalctx.Eval(block.Condition.Expr)
		if err != nil {
			return File{}, false, errors.E(ErrConditionEval, err)
		}
		if value.Type() != cty.Bool {
			return File{}, false, errors.E(
				ErrInvalidConditionType,
				"condition has type %s but must be boolean",
				value.Type().FriendlyName(),
			)
		}
		condition = value.True()
	}

	if !condition {
		return File{
			label:     name,
			origin:    block.Range,
			condition: condition,
			context:   block.Context,
		}, false, nil
	}

	inherit := true
	if block.Inherit != nil {
		value, err := evalctx.Eval(block.Inherit.Expr)
		if err != nil {
			return File{}, false, errors.E(ErrInheritEval, err)
		}

		if value.Type() != cty.Bool {
			return File{}, false, errors.E(
				ErrInvalidInheritType,
				`"inherit" has type %s but must be boolean`,
				value.Type().FriendlyName(),
			)
		}

		inherit = value.True()
	}

	if !inherit && block.Dir != cfg.Dir() {
		// ignore non-inheritable block
		return File{}, true, nil
	}

	asserts := make([]config.Assert, len(block.Asserts))
	assertsErrs := errors.L()
	assertFailed := false

	for i, assertCfg := range block.Asserts {
		assert, err := config.EvalAssert(evalctx, assertCfg)
		if err != nil {
			assertsErrs.Append(err)
			continue
		}
		asserts[i] = assert
		if !assert.Assertion && !assert.Warning {
			assertFailed = true
		}
	}

	if err := assertsErrs.AsError(); err != nil {
		return File{}, false, err
	}

	if assertFailed {
		return File{
			label:     name,
			origin:    block.Range,
			condition: condition,
			context:   block.Context,
			asserts:   asserts,
		}, false, nil
	}

	value, err := evalctx.Eval(block.Content.Expr)
	if err != nil {
		return File{}, false, errors.E(ErrContentEval, err)
	}

	if value.Type() != cty.String {
		return File{}, false, errors.E(
			ErrInvalidContentType,
			"content has type %s but must be string",
			value.Type().FriendlyName(),
		)
	}

	return File{
		label:     name,
		origin:    block.Range,
		body:      value.AsString(),
		condition: condition,
		context:   block.Context,
		asserts:   asserts,
	}, false, nil
}

// loadGenFileBlocks will load all generate_file blocks.
// The returned map maps the name of the block (its label)
// to the original block and the path (relative to project root) of the config
// from where it was parsed.
func loadGenFileBlocks(tree *config.Root, cfgdir project.Path) ([]hcl.GenFileBlock, error) {
	res := []hcl.GenFileBlock{}
	cfg, ok := tree.Lookup(cfgdir)
	if ok && !cfg.IsEmptyConfig() {
		res = append(res, cfg.Node.Generate.Files...)
	}

	parentCfgDir := cfgdir.Dir()
	if parentCfgDir == cfgdir {
		return res, nil
	}

	parentRes, err := loadGenFileBlocks(tree, parentCfgDir)
	if err != nil {
		return nil, err
	}

	res = append(res, parentRes...)
	return res, nil
}
