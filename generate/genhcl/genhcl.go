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

// Package genhcl implements generate_hcl code generation.
package genhcl

import (
	stdfmt "fmt"
	"sort"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/hcl/fmt"
	"github.com/mineiros-io/terramate/hcl/info"
	"github.com/mineiros-io/terramate/stack"

	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/lets"
	"github.com/mineiros-io/terramate/project"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
)

// HCL represents generated HCL code from a single block.
// Is contains parsed and evaluated code on it and information
// about the origin of the generated code.
type HCL struct {
	label     string
	origin    info.Range
	body      string
	condition bool
	asserts   []config.Assert
}

const (
	// Header is the current header string used by generate_hcl code generation.
	Header = "// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT"

	// HeaderV0 is the deprecated header string used by generate_hcl code generation.
	HeaderV0 = "// GENERATED BY TERRAMATE: DO NOT EDIT"
)

const (

	// ErrContentEval indicates the failure to evaluate the content block.
	ErrContentEval errors.Kind = "evaluating content block"

	// ErrConditionEval indicates the failure to evaluate the condition attribute.
	ErrConditionEval errors.Kind = "evaluating condition attribute"
)

// Label of the original generate_hcl block.
func (h HCL) Label() string {
	return h.label
}

// Asserts returns all (if any) of the evaluated assert configs of the
// generate_hcl block. If [HCL.Condition] returns false then assert configs
// will always be empty since they are not evaluated at all in that case.
func (h HCL) Asserts() []config.Assert {
	return h.asserts
}

// Header returns the header of the generated HCL file.
func (h HCL) Header() string {
	return stdfmt.Sprintf(
		"%s\n// TERRAMATE: originated from generate_hcl block on %s\n\n",
		Header,
		h.origin.Path(),
	)
}

// Body returns a string representation of the HCL code
// or an empty string if the config itself is empty.
func (h HCL) Body() string {
	return string(h.body)
}

// Range returns the range information of the generate_file block.
func (h HCL) Range() info.Range {
	return h.origin
}

// Condition returns the evaluated condition attribute for the generated code.
func (h HCL) Condition() bool {
	return h.condition
}

func (h HCL) Context() string {
	return "stack"
}

func (h HCL) String() string {
	return stdfmt.Sprintf("Generating file %q (condition %t) (body %q) (origin %q)",
		h.Label(), h.Condition(), h.Body(), h.Range().HostPath())
}

// Load loads from the file system all generate_hcl for a given stack.
// It will navigate the root configuration tree from the stack until it reaches
// the project /, loading generate_hcl and merging them appropriately.
//
// All generate_file blocks must have unique labels, even ones at different
// directories. Any conflicts will be reported as an error.
//
// Metadata and globals for the stack are used on the evaluation of the
// generate_hcl blocks.
//
// The rootdir MUST be an absolute path.
func Load(
	cfg *config.Tree,
	globalctx *eval.Context,
) ([]HCL, error) {
	logger := log.With().
		Str("action", "genhcl.Load()").
		Str("path", cfg.Dir()).
		Logger()

	logger.Trace().Msg("loading generate_hcl blocks.")

	hclBlocks, err := loadGenHCLBlocks(cfg.Root(), cfg.ProjDir())
	if err != nil {
		return nil, errors.E("loading generate_hcl", err)
	}

	logger.Trace().Msg("generating HCL code.")

	var hcls []HCL
	for _, hclBlock := range hclBlocks {
		evalctx := globalctx.Copy()

		name := hclBlock.Label
		err := lets.Load(hclBlock.Lets, evalctx)
		if err != nil {
			return nil, err
		}

		condition := true
		if hclBlock.Condition != nil {
			value, err := evalctx.Eval(hclBlock.Condition.Expr)
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
			hcls = append(hcls, HCL{
				label:     name,
				origin:    hclBlock.Range,
				condition: condition,
			})

			continue
		}

		asserts := make([]config.Assert, len(hclBlock.Asserts))
		assertsErrs := errors.L()
		assertFailed := false

		for i, assertCfg := range hclBlock.Asserts {
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
			hcls = append(hcls, HCL{
				label:     name,
				origin:    hclBlock.Range,
				condition: condition,
				asserts:   asserts,
			})
			continue
		}

		st, err := stack.New(cfg.RootDir(), cfg.Node)
		if err != nil {
			panic(err)
		}
		gen := hclwrite.NewEmptyFile()
		if err := copyBody(gen.Body(), hclBlock.Content.Body, evalctx); err != nil {
			return nil, errors.E(ErrContentEval, st, err,
				"generate_hcl %q", name,
			)
		}

		formatted, err := fmt.FormatMultiline(string(gen.Bytes()), hclBlock.Range.HostPath())
		if err != nil {
			panic(errors.E(st, err,
				"internal error: formatting generated code for generate_hcl %q:%s", name, string(gen.Bytes()),
			))
		}
		hcls = append(hcls, HCL{
			label:     name,
			origin:    hclBlock.Range,
			body:      formatted,
			condition: condition,
			asserts:   asserts,
		})
	}

	sort.SliceStable(hcls, func(i, j int) bool {
		return hcls[i].Label() < hcls[j].Label()
	})

	logger.Trace().Msg("evaluated all blocks with success")
	return hcls, nil
}

// loadGenHCLBlocks will load all generate_hcl blocks.
// The returned map maps the name of the block (its label)
// to the original block and the path (relative to project root) of the config
// from where it was parsed.
func loadGenHCLBlocks(tree *config.Root, cfgdir project.Path) ([]hcl.GenHCLBlock, error) {
	res := []hcl.GenHCLBlock{}
	cfg, ok := tree.Lookup(cfgdir)
	if ok && !cfg.IsEmptyConfig() {
		res = append(res, cfg.Node.Generate.HCLs...)
	}

	parentCfgDir := cfgdir.Dir()
	if parentCfgDir == cfgdir {
		return res, nil
	}

	parentRes, err := loadGenHCLBlocks(tree, parentCfgDir)
	if err != nil {
		return nil, err
	}

	res = append(res, parentRes...)

	return res, nil
}
