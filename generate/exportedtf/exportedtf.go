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

package exportedtf

import (
	"fmt"
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

// StackTf represents all exported terraform code for a stack,
// mapping the exported code name to the actual Terraform code.
type StackTf struct {
	tfcode map[string]Body
}

// Body represents exported Terraform code from a single block.
// Is contains parsed and evaluated code on it.
type Body struct {
	body []byte
}

const (
	ErrInvalidBlock errutil.Error = "invalid export_as_terraform block"
)

// ExportedCode returns all exported code, mapping the name to its
// equivalent generated code.
func (s StackTf) ExportedCode() map[string]Body {
	cp := map[string]Body{}
	for k, v := range s.tfcode {
		cp[k] = v
	}
	return cp
}

// String returns a string representation of the Terraform code
// or an empty string if the config itself is empty.
func (b Body) String() string {
	return string(b.body)
}

// Load loads from the file system all export_as_terraform for
// a given stack. It will navigate the file system from the stack dir until
// it reaches rootdir, loading export_as_terraform and merging them appropriately.
//
// More specific definitions (closer or at the stack) have precedence over
// less specific ones (closer or at the root dir).
//
// Metadata and globals for the stack are used on the evaluation of the
// export_as_terramate blocks.
//
// The returned result only contains evaluated values.
//
// The rootdir MUST be an absolute path.
func Load(rootdir string, sm stack.Metadata, globals *terramate.Globals) (StackTf, error) {
	stackpath := filepath.Join(rootdir, sm.Path)
	logger := log.With().
		Str("action", "exportedtf.Load()").
		Str("path", stackpath).
		Logger()

	logger.Trace().Msg("loading export_as_terraform blocks.")

	exportBlocks, err := loadExportBlocks(rootdir, stackpath)
	if err != nil {
		return StackTf{}, fmt.Errorf("loading exported terraform code: %w", err)
	}

	evalctx, err := newEvalCtx(stackpath, sm, globals)
	if err != nil {
		return StackTf{}, fmt.Errorf("preparing to eval exported terraform code: %v", err)
	}

	logger.Trace().Msg("generating exported terraform code.")

	res := StackTf{
		tfcode: map[string]Body{},
	}

	for name, block := range exportBlocks {
		logger := logger.With().
			Str("block", name).
			Logger()

		logger.Trace().Msg("evaluating block.")

		gen := hclwrite.NewEmptyFile()
		if err := hcl.CopyBody(gen.Body(), block.Body, evalctx); err != nil {
			return StackTf{}, fmt.Errorf(
				"generating terraform code for stack %q block %q: %v",
				stackpath,
				name,
				err,
			)
		}
		res.tfcode[name] = Body{body: gen.Bytes()}
	}

	return res, nil
}

func newEvalCtx(stackpath string, sm stack.Metadata, globals *terramate.Globals) (*eval.Context, error) {
	logger := log.With().
		Str("action", "exportedtf.newEvalCtx()").
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

// loadExportBlocks will load all export_as_terraform blocks applying overriding
// as it goes, the returned map maps the name of the block (its label) to the original block
func loadExportBlocks(rootdir string, cfgdir string) (map[string]*hclsyntax.Block, error) {
	logger := log.With().
		Str("action", "exportedtf.loadExportBlocks()").
		Str("root", rootdir).
		Str("configDir", cfgdir).
		Logger()

	logger.Trace().Msg("Parsing export_as_terraform blocks.")

	if !strings.HasPrefix(cfgdir, rootdir) {
		logger.Trace().Msg("config dir outside root, nothing to do")
		return nil, nil
	}

	cfgpath := filepath.Join(cfgdir, config.Filename)
	blocks, err := hcl.ParseExportAsTerraformBlocks(cfgpath)
	if err != nil {
		return nil, fmt.Errorf("loading exported terraform code: %v", err)
	}

	res := map[string]*hclsyntax.Block{}

	for _, block := range blocks {
		// TODO(katcipis): properly test two blocks with same label on same config file
		if len(block.Labels) != 1 {
			return nil, fmt.Errorf(
				"%w: want single label instead got %d",
				ErrInvalidBlock,
				len(block.Labels),
			)
		}
		name := block.Labels[0]
		res[name] = block
	}

	// TODO(katcipis): Handle failure on parent configs
	parentRes, _ := loadExportBlocks(rootdir, filepath.Dir(cfgdir))

	merge(res, parentRes)
	return res, nil
}

func merge(target, src map[string]*hclsyntax.Block) {
	for k, v := range src {
		if _, ok := target[k]; ok {
			continue
		}
		target[k] = v
	}
}
