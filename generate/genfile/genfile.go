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
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/hcl/eval"

	"github.com/mineiros-io/terramate/lets"
	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/stack"
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
	origin    project.Path
	body      string
	condition bool
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

// Origin returns the path, relative to the project root,
// of the configuration that originated the file.
func (f File) Origin() project.Path {
	return f.origin
}

// Condition returns the result of the evaluation of the
// condition attribute for the generated code.
func (f File) Condition() bool {
	return f.condition
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
		f.Label(), f.Condition(), f.Body(), f.Origin())
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
	cfg *config.Tree,
	projmeta project.Metadata,
	sm stack.Metadata,
	globals *eval.Object,
) ([]File, error) {
	genFileBlocks, err := loadGenFileBlocks(cfg, sm.Path())
	if err != nil {
		return nil, errors.E("loading generate_file", err)
	}

	var files []File

	for _, genFileBlock := range genFileBlocks {
		name := genFileBlock.Label
		origin := project.PrjAbsPath(cfg.RootDir(), genFileBlock.Origin)
		evalctx := stack.NewEvalCtx(projmeta, sm, globals)
		err := lets.Load(genFileBlock.Lets, evalctx.Context)
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
				origin:    origin,
				condition: condition,
			})
			continue
		}

		//asserts := make([]config.Assert, len(genFileBlock.Asserts))
		//assertsErrs := errors.L()
		//assertFailed := false

		//for _, assertCfg := range genFileBlock.Asserts {
		//assert, err := config.EvalAssert(evalctx.Context, assertCfg)
		////if err != nil {
		////assertsErrs.Append(err)
		////continue
		////}
		//asserts[i] = assert
		//if !assert.Assertion && !assert.Warning {
		//assertFailed = true
		//}
		//}

		//if err := assertsErrs.AsError(); err != nil {
		//return nil, err
		//}

		//if assertFailed {
		//hcls = append(hcls, HCL{
		//label:     name,
		//origin:    origin,
		//condition: condition,
		//asserts:   asserts,
		//})
		//continue
		//}

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
			origin:    origin,
			body:      value.AsString(),
			condition: condition,
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].String() < files[j].String()
	})

	return files, nil
}

// loadGenFileBlocks will load all generate_file blocks.
// The returned map maps the name of the block (its label)
// to the original block and the path (relative to project root) of the config
// from where it was parsed.
func loadGenFileBlocks(tree *config.Tree, cfgdir project.Path) ([]hcl.GenFileBlock, error) {
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
