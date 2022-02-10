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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/madlambda/spells/errutil"
	"github.com/mineiros-io/terramate"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/hcl/eval"
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
	body   []byte
}

const (
	ErrMultiLevelConflict errutil.Error = "conflicting generate_hcl blocks"
	ErrInvalidBlock       errutil.Error = "invalid generate_hcl block"
	ErrEval               errutil.Error = "evaluating generate_hcl block"
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

// String returns a string representation of the HCL code
// or an empty string if the config itself is empty.
func (b HCL) String() string {
	return string(b.body)
}

// Origin returns the path, relative to the project root,
// of the configuration that originated the code.
func (b HCL) Origin() string {
	return b.origin
}

// Load loads from the file system all generate_hcl for
// a given stack. It will navigate the file system from the stack dir until
// it reaches rootdir, loading generate_hcl and merging them appropriately.
//
// More specific definitions (closer or at the stack) have precedence over
// less specific ones (closer or at the root dir).
//
// Metadata and globals for the stack are used on the evaluation of the
// generate_hcl blocks.
//
// The returned result only contains evaluated values.
//
// The rootdir MUST be an absolute path.
func Load(rootdir string, sm stack.Metadata, globals *terramate.Globals) (StackHCLs, error) {
	stackpath := filepath.Join(rootdir, sm.Path)
	logger := log.With().
		Str("action", "genhcl.Load()").
		Str("path", stackpath).
		Logger()

	logger.Trace().Msg("loading generate_hcl blocks.")

	loadedHCLs, err := loadGenHCLBlocks(rootdir, stackpath)
	if err != nil {
		return StackHCLs{}, fmt.Errorf("loading generate_hcl: %w", err)
	}

	evalctx, err := newEvalCtx(stackpath, sm, globals)
	if err != nil {
		return StackHCLs{}, fmt.Errorf("%w: creating eval context: %v", ErrEval, err)
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
			return StackHCLs{}, fmt.Errorf(
				"%w: stack %q block %q: %v",
				ErrEval,
				stackpath,
				name,
				err,
			)
		}
		res.hcls[name] = HCL{
			origin: loadedHCL.origin,
			body:   gen.Bytes(),
		}
	}

	return res, nil
}

func newEvalCtx(stackpath string, sm stack.Metadata, globals *terramate.Globals) (*eval.Context, error) {
	logger := log.With().
		Str("action", "genhcl.newEvalCtx()").
		Str("path", stackpath).
		Logger()

	evalctx := eval.NewContext(stackpath)

	logger.Trace().Msg("Add stack metadata evaluation namespace.")

	err := evalctx.SetNamespace("terramate", sm.ToCtyMap())
	if err != nil {
		return nil, fmt.Errorf("setting terramate namespace on eval context for stack %q: %v",
			stackpath, err)
	}

	logger.Trace().Msg("Add global evaluation namespace.")

	if err := evalctx.SetNamespace("global", globals.Attributes()); err != nil {
		return nil, fmt.Errorf("setting global namespace on eval context for stack %q: %v",
			stackpath, err)
	}

	return evalctx, nil
}

type loadedHCL struct {
	origin string
	block  *hclsyntax.Block
}

// loadGenHCLBlocks will load all generate_hcl blocks applying overriding
// as it goes, the returned map maps the name of the block (its label)
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

	cfgpath := filepath.Join(cfgdir, config.Filename)
	blocks, err := hcl.ParseGenerateHCLBlocks(cfgpath)
	if err != nil {
		if os.IsNotExist(err) {
			return loadGenHCLBlocks(rootdir, filepath.Dir(cfgdir))
		}
		return nil, fmt.Errorf("parsing generate_hcl code: %v", err)
	}

	res := map[string]loadedHCL{}

	for _, block := range blocks {
		if err := validateGenerateHCLBlock(block); err != nil {
			return nil, fmt.Errorf("%w:%v", ErrInvalidBlock, err)
		}
		name := block.Labels[0]
		if _, ok := res[name]; ok {
			return nil, fmt.Errorf(
				"%w: found two blocks with same label %q",
				ErrInvalidBlock,
				name,
			)
		}
		res[name] = loadedHCL{
			origin: strings.TrimPrefix(cfgpath, rootdir),
			block:  block,
		}
	}

	parentRes, err := loadGenHCLBlocks(rootdir, filepath.Dir(cfgdir))
	if err != nil {
		return nil, err
	}
	if err := join(res, parentRes); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMultiLevelConflict, err)
	}
	return res, nil
}

func validateGenerateHCLBlock(block *hclsyntax.Block) error {
	if len(block.Labels) != 1 {
		return fmt.Errorf(
			"want single label instead got %d",
			len(block.Labels),
		)
	}
	if block.Labels[0] == "" {
		return errors.New("label can't be empty")
	}
	if len(block.Body.Attributes) != 0 {
		return errors.New("attributes are not allowed")
	}
	return nil
}

func join(target, src map[string]loadedHCL) error {
	for blockLabel, srcHCL := range src {
		if targetHCL, ok := target[blockLabel]; ok {
			return fmt.Errorf(
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
