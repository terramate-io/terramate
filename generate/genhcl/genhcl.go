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
	"strings"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/stack"
	"github.com/rs/zerolog/log"
)

// StackHCLs represents all generated HCL code for a stack,
// mapping the generated code filename to the actual HCL code.
type StackHCLs struct {
	hcls map[string]HCL
}

// HCL represents generated HCL code from a single block.
// Is contains parsed and evaluated code on it and information
// about the origin of the generated code.
type HCL struct {
	origin string
	body   string
}

const (
	// Header is the current header string used by generate_hcl code generation.
	Header = "// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT"

	// HeaderV0 is the deprecated header string used by generate_hcl code generation.
	HeaderV0 = "// GENERATED BY TERRAMATE: DO NOT EDIT"
)

const (
	// ErrLabelConflict indicates the two generate_hcl blocks
	// have the same label.
	ErrLabelConflict errors.Kind = "label conflict detected"

	// ErrParsing indicates the failure of parsing the generate_hcl block.
	ErrParsing errors.Kind = "parsing generate_hcl block"

	// ErrEvalContent indicates the failure to evaluate the content block.
	ErrEvalContent errors.Kind = "evaluating content block"

	// ErrEvalCondition indicates the failure to evaluate the condition attribute.
	ErrEvalCondition errors.Kind = "evaluating condition attribute"
)

// GeneratedHCLs returns all generated code, mapping the name to its
// equivalent generated code.
func (s StackHCLs) GeneratedHCLs() map[string]HCL {
	cp := map[string]HCL{}
	for k, v := range s.hcls {
		cp[k] = v
	}
	return cp
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

// Condition returns the result of the evaluation of the
// condition attribute for the generated code.
func (h HCL) Condition() bool {
	return true
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
func Load(rootdir string, sm stack.Metadata, globals stack.Globals) (StackHCLs, error) {
	stackpath := filepath.Join(rootdir, sm.Path())
	logger := log.With().
		Str("action", "genhcl.Load()").
		Str("path", stackpath).
		Logger()

	logger.Trace().Msg("loading generate_hcl blocks.")

	loadedHCLs, err := loadGenHCLBlocks(rootdir, stackpath)
	if err != nil {
		return StackHCLs{}, errors.E("loading generate_hcl", err)
	}

	evalctx, err := stack.NewEvalCtx(stackpath, sm, globals)
	if err != nil {
		return StackHCLs{}, errors.E(err, "creating eval context")
	}

	logger.Trace().Msg("generating HCL code.")

	res := StackHCLs{
		hcls: map[string]HCL{},
	}

	for name, loadedHCL := range loadedHCLs {
		logger := logger.With().
			Str("block", name).
			Logger()

		logger.Trace().Msg("evaluating block.")

		gen := hclwrite.NewEmptyFile()
		if err := hcl.CopyBody(gen.Body(), loadedHCL.block.Body, evalctx); err != nil {
			return StackHCLs{}, errors.E(ErrEvalContent, sm, err,
				"failed to generate block %q", name,
			)
		}
		formatted, err := hcl.Format(string(gen.Bytes()), loadedHCL.origin)
		if err != nil {
			return StackHCLs{}, errors.E(sm, err,
				"failed to format generated code for block %q", name,
			)
		}
		res.hcls[name] = HCL{
			origin: loadedHCL.origin,
			body:   formatted,
		}
	}

	logger.Trace().Msg("evaluated all blocks with success.")
	return res, nil
}

type loadedHCL struct {
	origin string
	block  *hclsyntax.Block
}

// loadGenHCLBlocks will load all generate_hcl blocks.
// The returned map maps the name of the block (its label)
// to the original block and the path (relative to project root) of the config
// from where it was parsed.
func loadGenHCLBlocks(rootdir string, cfgdir string) (map[string]loadedHCL, error) {
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

	hclblocks, err := hcl.ParseGenerateHCLBlocks(cfgdir)
	if err != nil {
		return nil, errors.E(ErrParsing, err, "cfgdir %q", cfgdir)
	}

	logger.Trace().Msg("Parsed generate_hcl blocks.")
	res := map[string]loadedHCL{}

	for filename, genhclBlocks := range hclblocks {
		for _, genhclBlock := range genhclBlocks {
			name := genhclBlock.Label
			origin := project.PrjAbsPath(rootdir, filename)

			if other, ok := res[name]; ok {
				return nil, conflictErr(name, origin, other.origin)
			}

			res[name] = loadedHCL{
				origin: project.PrjAbsPath(rootdir, filename),
				block:  genhclBlock.Content,
			}

			logger.Trace().Msg("loaded generate_hcl block.")
		}
	}

	parentRes, err := loadGenHCLBlocks(rootdir, filepath.Dir(cfgdir))
	if err != nil {
		return nil, err
	}
	if err := join(res, parentRes); err != nil {
		return nil, errors.E(ErrLabelConflict, err)
	}

	logger.Trace().Msg("loaded generate_hcl blocks with success.")
	return res, nil
}

func join(target, src map[string]loadedHCL) error {
	for blockLabel, srcHCL := range src {
		if targetHCL, ok := target[blockLabel]; ok {
			return errors.E(
				"found label %q at %q and %q",
				blockLabel,
				srcHCL.origin,
				targetHCL.origin,
			)
		}
		target[blockLabel] = srcHCL
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
