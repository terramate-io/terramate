// Copyright 2022 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package genfile implements generate_file code generation.
package genfile

import (
	"fmt"
	"sort"

	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/hcl/info"

	"github.com/mineiros-io/terramate/lets"
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

	// ErrLabelConflict indicates the two generate_file blocks
	// have the same label.
	ErrLabelConflict errors.Kind = "label conflict detected"
)

// File represents generated file from a single generate_file block.
type File struct {
	label     string
	origin    info.Range
	body      string
	condition bool
	scope     string
	context   string
	asserts   []config.Assert
}

// Label of the original generate_file block.
func (f File) Label() string {
	return f.label
}

// Body returns the file body.
func (f File) Body() string {
	return f.body
}

// Scope returns the file's scope.
func (f File) Scope() string {
	return f.scope
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

// Context returns the result of the evaluation of the context attribute.
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
	return fmt.Sprintf("generate_file %q (condition %t) (context %s) (body %q) (origin %q)",
		f.Label(), f.Condition(), f.Context(), f.Body(), f.Range().Path())
}

// LoadStackContext loads and parses from the file system all generate_file
// blocks for a given stack. It will navigate the file system from the stack dir
// until it reaches rootdir, loading generate_file blocks found on Terramate
// configuration files.
//
// All generate_file blocks must have unique labels, even ones at different
// directories. Any conflicts will be reported as an error.
//
// Metadata and globals for the stack are used on the evaluation of the
// generate_file blocks.
//
// The rootdir MUST be an absolute path.
func LoadStackContext(
	cfg *config.Tree,
	evalctx *eval.Context,
) ([]File, error) {
	genFileBlocks := cfg.UpwardGenerateFiles()

	var files []File

	for _, genFileBlock := range genFileBlocks {
		name := genFileBlock.Label

		context := "stack"
		if genFileBlock.Context != nil {
			val, err := evalctx.Eval(genFileBlock.Context.Expr)
			if err != nil {
				return nil, errors.E(
					genFileBlock.Range,
					err,
					"failed to evaluate genfile context",
				)
			}
			if val.Type() != cty.String {
				return nil, errors.E(
					"generate_file.context must be a string but given %s",
					val.Type().FriendlyName(),
				)
			}
			context = val.AsString()
		}

		// only handle stack context here.
		if context != "stack" {
			continue
		}

		err := lets.Load(genFileBlock.Lets, evalctx)
		if err != nil {
			return nil, err
		}

		condition := true
		if genFileBlock.Condition != nil {
			value, err := evalctx.Eval(genFileBlock.Condition.Expr)
			if err != nil {
				return nil, errors.E(ErrConditionEval, err)
			}
			if value.Type() != cty.Bool {
				return nil, errors.E(
					ErrInvalidConditionType,
					"condition has type %s but must be boolean",
					value.Type().FriendlyName(),
				)
			}
			condition = value.True()
		}

		if !condition {
			files = append(files, File{
				label:     name,
				origin:    genFileBlock.Range,
				condition: condition,
				context:   context,
			})
			continue
		}

		asserts := make([]config.Assert, len(genFileBlock.Asserts))
		assertsErrs := errors.L()
		assertFailed := false

		for i, assertCfg := range genFileBlock.Asserts {
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
			return nil, err
		}

		if assertFailed {
			files = append(files, File{
				label:     name,
				origin:    genFileBlock.Range,
				condition: condition,
				asserts:   asserts,
			})
			continue
		}

		value, err := evalctx.Eval(genFileBlock.Content.Expr)
		if err != nil {
			return nil, errors.E(ErrContentEval, err)
		}

		if value.Type() != cty.String {
			return nil, errors.E(
				ErrInvalidContentType,
				"content has type %s but must be string",
				value.Type().FriendlyName(),
			)
		}

		files = append(files, File{
			label:     name,
			origin:    genFileBlock.Range,
			body:      value.AsString(),
			condition: condition,
			asserts:   asserts,
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].String() < files[j].String()
	})

	return files, nil
}
