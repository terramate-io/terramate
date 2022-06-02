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

package genfile

import (
	"path/filepath"
	"strings"

	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/stack"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
)

const (
	// ErrInvalidContentType indicates the content attribute
	// has a invalid type.
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

// StackFiles represents all generated files for a stack,
// mapping the generated file path to the actual file body.
type StackFiles struct {
	files map[string]File
}

// File represents generated file from a single generate_file block.
type File struct {
	origin    string
	body      string
	condition bool
}

// Body returns the file body.
func (f File) Body() string {
	return f.body
}

// Origin returns the path, relative to the project root,
// of the configuration that originated the file.
func (f File) Origin() string {
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

// GeneratedFiles returns all generated files, mapping the file path to
// the file description. The path is absolute relative to the project root.
func (s StackFiles) GeneratedFiles() map[string]File {
	cp := map[string]File{}
	for k, v := range s.files {
		cp[k] = v
	}
	return cp
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
func Load(rootdir string, sm stack.Metadata, globals stack.Globals) (StackFiles, error) {
	stackpath := filepath.Join(rootdir, sm.Path())
	logger := log.With().
		Str("action", "genfile.Load()").
		Str("path", stackpath).
		Logger()

	logger.Trace().Msg("loading generate_file blocks")

	genFileBlocks, err := loadGenFileBlocks(rootdir, stackpath)
	if err != nil {
		return StackFiles{}, errors.E("loading generate_file", err)
	}

	evalctx, err := stack.NewEvalCtx(stackpath, sm, globals)
	if err != nil {
		return StackFiles{}, errors.E(err, "creating eval context")
	}

	logger.Trace().Msg("generating files")

	res := StackFiles{
		files: map[string]File{},
	}

	for name, genFileBlock := range genFileBlocks {
		logger := logger.With().
			Str("block", name).
			Logger()

		logger.Trace().Msg("evaluating contents")

		value, err := evalctx.Eval(genFileBlock.block.Content.Expr)
		if err != nil {
			return StackFiles{}, errors.E(ErrContentEval, err)
		}

		condition := true
		if genFileBlock.block.Condition != nil {
			logger.Trace().Msg("has condition attribute, evaluating it")
			value, err := evalctx.Eval(genFileBlock.block.Condition.Expr)
			if err != nil {
				return StackFiles{}, errors.E(ErrConditionEval, err)
			}
			if value.Type() != cty.Bool {
				return StackFiles{}, errors.E(
					ErrInvalidConditionType,
					"condition has type %s but must be boolean",
					value.Type().FriendlyName(),
				)
			}
			condition = value.True()
		}

		if value.Type() != cty.String {
			return StackFiles{}, errors.E(
				ErrInvalidContentType,
				"content has type %s but must be string",
				value.Type().FriendlyName(),
			)
		}

		res.files[name] = File{
			origin:    genFileBlock.origin,
			body:      value.AsString(),
			condition: condition,
		}
	}

	logger.Trace().Msg("evaluated all blocks with success.")

	return res, nil
}

type genFileBlock struct {
	origin string
	block  hcl.GenFileBlock
}

// loadGenFileBlocks will load all generate_file blocks.
// The returned map maps the name of the block (its label)
// to the original block and the path (relative to project root) of the config
// from where it was parsed.
func loadGenFileBlocks(rootdir string, cfgdir string) (map[string]genFileBlock, error) {
	logger := log.With().
		Str("action", "genfile.loadGenFileBlocks()").
		Str("root", rootdir).
		Str("configDir", cfgdir).
		Logger()

	logger.Trace().Msg("Parsing generate_hcl blocks.")

	if !strings.HasPrefix(cfgdir, rootdir) {
		logger.Trace().Msg("config dir outside root, nothing to do")
		return nil, nil
	}

	genFileBlocks, err := hcl.ParseGenerateFileBlocks(cfgdir)
	if err != nil {
		return nil, errors.E(err, "cfgdir %q", cfgdir)
	}

	logger.Trace().Msg("Parsed generate_file blocks.")
	res := map[string]genFileBlock{}

	for filename, blocks := range genFileBlocks {
		for _, block := range blocks {
			name := block.Label
			origin := project.PrjAbsPath(rootdir, filename)

			if other, ok := res[name]; ok {
				return nil, conflictErr(name, origin, other.origin)
			}

			res[name] = genFileBlock{
				origin: origin,
				block:  block,
			}

			logger.Trace().Msg("loaded generate_file block.")
		}
	}

	parentRes, err := loadGenFileBlocks(rootdir, filepath.Dir(cfgdir))
	if err != nil {
		return nil, err
	}

	if err := merge(res, parentRes); err != nil {
		return nil, err
	}

	logger.Trace().Msg("loaded generate_file blocks with success.")
	return res, nil
}

func merge(target, src map[string]genFileBlock) error {
	for blockLabel, srcFile := range src {
		if targetFile, ok := target[blockLabel]; ok {
			return conflictErr(blockLabel, srcFile.origin, targetFile.origin)
		}
		target[blockLabel] = srcFile
	}
	return nil
}

func conflictErr(label, origin, otherOrigin string) error {
	if origin == otherOrigin {
		return errors.E(
			ErrLabelConflict,
			"%s has blocks with same label %q",
			origin,
			label,
		)
	}
	return errors.E(
		ErrLabelConflict,
		"%s and %s have blocks with same label %q",
		origin,
		otherOrigin,
		label,
	)
}
