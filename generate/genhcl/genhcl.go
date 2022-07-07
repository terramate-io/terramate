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

package genhcl

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/stack"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
)

// HCL represents generated HCL code from a single block.
// Is contains parsed and evaluated code on it and information
// about the origin of the generated code.
type HCL struct {
	name      string
	origin    string
	body      string
	condition bool
}

const (
	// Header is the current header string used by generate_hcl code generation.
	Header = "// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT"

	// HeaderV0 is the deprecated header string used by generate_hcl code generation.
	HeaderV0 = "// GENERATED BY TERRAMATE: DO NOT EDIT"
)

const (
	// ErrParsing indicates the failure of parsing the generate_hcl block.
	ErrParsing errors.Kind = "parsing generate_hcl block"

	// ErrContentEval indicates the failure to evaluate the content block.
	ErrContentEval errors.Kind = "evaluating content block"

	// ErrConditionEval indicates the failure to evaluate the condition attribute.
	ErrConditionEval errors.Kind = "evaluating condition attribute"

	// ErrInvalidConditionType indicates the condition attribute
	// has an invalid type.
	ErrInvalidConditionType errors.Kind = "invalid condition type"
)

// Name of the HCL code.
func (h HCL) Name() string {
	return h.name
}

// Header returns the header of the generated HCL file.
func (h HCL) Header() string {
	return fmt.Sprintf(
		"%s\n// TERRAMATE: originated from generate_hcl block on %s\n\n",
		Header,
		h.origin,
	)
}

// Body returns a string representation of the HCL code
// or an empty string if the config itself is empty.
func (h HCL) Body() string {
	return string(h.body)
}

// Origin returns the path, relative to the project root,
// of the configuration that originated the code.
func (h HCL) Origin() string {
	return h.origin
}

// Condition returns the evaluated condition attribute for the generated code.
func (h HCL) Condition() bool {
	return h.condition
}

func (h HCL) String() string {
	return fmt.Sprintf("Generating file %q (condition %t) (body %q) (origin %q)",
		h.Name(), h.Condition(), h.Body(), h.Origin())
}

// Load loads from the file system all generate_hcl for
// a given stack. It will navigate the file system from the stack dir until
// it reaches rootdir, loading generate_hcl and merging them appropriately.
//
// All generate_file blocks must have unique labels, even ones at different
// directories. Any conflicts will be reported as an error.
//
// Metadata and globals for the stack are used on the evaluation of the
// generate_hcl blocks.
//
// The rootdir MUST be an absolute path.
func Load(rootdir string, sm stack.Metadata, globals stack.Globals) ([]HCL, error) {
	logger := log.With().
		Str("action", "genhcl.Load()").
		Str("path", sm.HostPath()).
		Logger()

	logger.Trace().Msg("loading generate_hcl blocks.")

	loadedHCLs, err := loadGenHCLBlocks(rootdir, sm.HostPath())
	if err != nil {
		return nil, errors.E("loading generate_hcl", err)
	}

	evalctx := stack.NewEvalCtx(rootdir, sm, globals)

	logger.Trace().Msg("generating HCL code.")

	var hcls []HCL
	for _, loadedHCL := range loadedHCLs {
		name := loadedHCL.name

		logger := logger.With().
			Str("block", name).
			Logger()

		condition := true
		if loadedHCL.condition != nil {
			logger.Trace().Msg("has condition attribute, evaluating it")
			value, err := evalctx.Eval(loadedHCL.condition.Expr)
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
			logger.Trace().Msg("condition=false, block wont be evaluated")

			hcls = append(hcls, HCL{
				name:      name,
				origin:    loadedHCL.origin,
				condition: condition,
			})

			continue
		}

		logger.Trace().Msg("evaluating block")

		gen := hclwrite.NewEmptyFile()
		if err := hcl.CopyBody(gen.Body(), loadedHCL.block.Body, evalctx.PartialEval); err != nil {
			return nil, errors.E(ErrContentEval, sm, err,
				"failed to generate block %q", name,
			)
		}
		formatted, err := hcl.FormatMultiline(string(gen.Bytes()), loadedHCL.origin)
		if err != nil {
			return nil, errors.E(sm, err,
				"failed to format generated code for block %q", name,
			)
		}
		hcls = append(hcls, HCL{
			name:      name,
			origin:    loadedHCL.origin,
			body:      formatted,
			condition: condition,
		})
	}

	sort.Slice(hcls, func(i, j int) bool {
		return hcls[i].String() < hcls[j].String()
	})

	logger.Trace().Msg("evaluated all blocks with success")
	return hcls, nil
}

type loadedHCL struct {
	name      string
	origin    string
	block     *hclsyntax.Block
	condition *hclsyntax.Attribute
}

// loadGenHCLBlocks will load all generate_hcl blocks.
// The returned map maps the name of the block (its label)
// to the original block and the path (relative to project root) of the config
// from where it was parsed.
func loadGenHCLBlocks(rootdir string, cfgdir string) ([]loadedHCL, error) {
	logger := log.With().
		Str("action", "genhcl.loadGenHCLBlocks()").
		Str("root", rootdir).
		Str("configDir", cfgdir).
		Logger()

	logger.Trace().Msg("Parsing generate_hcl blocks.")

	if !strings.HasPrefix(cfgdir, rootdir) {
		logger.Trace().Msg("config dir outside root, nothing to do")
		return nil, nil
	}

	blocks, err := hcl.ParseGenerateHCLBlocks(rootdir, cfgdir)
	if err != nil {
		return nil, errors.E(ErrParsing, err, "cfgdir %q", cfgdir)
	}

	logger.Trace().Msg("Parsed generate_hcl blocks.")
	res := []loadedHCL{}

	for _, genhclBlock := range blocks {
		name := genhclBlock.Label
		origin := project.PrjAbsPath(rootdir, genhclBlock.Origin)

		res = append(res, loadedHCL{
			name:      name,
			origin:    origin,
			block:     genhclBlock.Content,
			condition: genhclBlock.Condition,
		})

		logger.Trace().Msg("loaded generate_hcl block.")
	}

	parentCfgDir := filepath.Dir(cfgdir)
	if parentCfgDir == cfgdir {
		return res, nil
	}

	parentRes, err := loadGenHCLBlocks(rootdir, parentCfgDir)
	if err != nil {
		return nil, err
	}

	res = append(res, parentRes...)

	logger.Trace().Msg("loaded generate_hcl blocks with success.")
	return res, nil
}
