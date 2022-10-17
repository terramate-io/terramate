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

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	mcty "github.com/mineiros-io/terramate/hcl/cty"
	"github.com/mineiros-io/terramate/lets"
	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/stack"
	"github.com/rs/zerolog/log"
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
func Load(cfg *config.Tree, projmeta project.Metadata, sm stack.Metadata, globals *mcty.Object) ([]File, error) {
	logger := log.With().
		Str("action", "genfile.Load()").
		Str("path", sm.HostPath()).
		Logger()

	logger.Trace().Msg("loading generate_file blocks")

	genFileBlocks, err := loadGenFileBlocks(cfg, sm.Path())
	if err != nil {
		return nil, errors.E("loading generate_file", err)
	}

	logger.Trace().Msg("generating files")

	var files []File

	for _, genFileBlock := range genFileBlocks {
		name := genFileBlock.label
		origin := genFileBlock.origin

		logger := logger.With().
			Str("block", name).
			Stringer("origin", origin).
			Logger()

		evalctx := stack.NewEvalCtx(projmeta, sm, globals)
		err := lets.Load(genFileBlock.lets, evalctx.Context)
		if err != nil {
			return nil, err
		}

		logger.Trace().Msg("evaluating condition")

		condition := true
		if genFileBlock.block.Condition != nil {
			logger.Trace().Msg("has condition attribute, evaluating it")
			value, err := evalctx.Eval(genFileBlock.block.Condition.Expr)
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
			logger.Trace().Msg("condition=false, content wont be evaluated")

			files = append(files, File{
				label:     name,
				origin:    genFileBlock.origin,
				condition: condition,
			})

			continue
		}

		logger.Trace().Msg("evaluating contents")

		value, err := evalctx.Eval(genFileBlock.block.Content.Expr)
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
			origin:    genFileBlock.origin,
			body:      value.AsString(),
			condition: condition,
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].String() < files[j].String()
	})

	logger.Trace().Msg("evaluated all blocks with success.")

	return files, nil
}

type genFileBlock struct {
	label  string
	origin project.Path
	lets   hclsyntax.Blocks
	block  hcl.GenFileBlock
}

// loadGenFileBlocks will load all generate_file blocks.
// The returned map maps the name of the block (its label)
// to the original block and the path (relative to project root) of the config
// from where it was parsed.
func loadGenFileBlocks(tree *config.Tree, cfgdir project.Path) ([]genFileBlock, error) {
	logger := log.With().
		Str("action", "genfile.loadGenFileBlocks()").
		Str("root", tree.RootDir()).
		Stringer("configDir", cfgdir).
		Logger()

	logger.Trace().Msg("Loading generate_hcl blocks.")

	res := []genFileBlock{}
	cfg, ok := tree.Lookup(cfgdir)
	if ok && !cfg.IsEmptyConfig() {
		blocks := cfg.Node.Generate.Files

		logger.Trace().Msg("Parsed generate_file blocks.")

		for _, block := range blocks {
			origin := project.PrjAbsPath(tree.RootDir(), block.Origin)

			res = append(res, genFileBlock{
				label:  block.Label,
				origin: origin,
				block:  block,
				lets:   block.Lets,
			})

			logger.Trace().Msg("loaded generate_file block.")
		}
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

	logger.Trace().Msg("loaded generate_file blocks with success.")
	return res, nil
}
