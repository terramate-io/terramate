// Copyright 2021 Mineiros GmbH
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

package terramate

import (
	"fmt"
	"path/filepath"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/madlambda/spells/errutil"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/stack"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
)

const (
	ErrExportedLocalRedefined errutil.Error = "export_as_locals attribute redefined"
	ErrExportedLocalsParsing  errutil.Error = "parsing export_as_locals"
)

// ExportedLocalValues represents information exported from Terramate
// in a way that is suitable to be used for Terraform code generation.
type ExportedLocalValues struct {
	attributes map[string]cty.Value
}

// LoadStackExportedLocals loads from the file system all export_as_locals for
// a given stack. It will navigate the file system from the stack dir until
// it reaches rootdir, loading export_as_locals and merging them appropriately.
//
// More specific definitions (closer or at the stack) have precedence over
// less specific ones (closer or at the root dir).
//
// Metadata and globals for the stack are used on the evaluation of the
// export_as_locals blocks.
//
// The rootdir MUST be an absolute path.
func LoadStackExportedLocals(rootdir string, sm stack.Metadata, g Globals) (ExportedLocalValues, error) {
	logger := log.With().
		Str("action", "LoadStackExportedLocals()").
		Str("stack", sm.Path).
		Logger()

	logger.Trace().
		Str("path", rootdir).
		Msg("Get absolute file path of rootdir.")
	if !filepath.IsAbs(rootdir) {
		return ExportedLocalValues{}, fmt.Errorf("%q must be an absolute path", rootdir)
	}

	logger.Trace().
		Str("path", rootdir).
		Msg("Load stack exported locals exprs.")
	localVars, err := loadStackExportedLocalExprs(rootdir, sm.Path)
	if err != nil {
		return ExportedLocalValues{}, err
	}
	return localVars.eval(sm, g)
}

func (e ExportedLocalValues) Attributes() map[string]cty.Value {
	attrcopy := map[string]cty.Value{}
	for k, v := range e.attributes {
		attrcopy[k] = v
	}
	return attrcopy
}

func loadStackExportedLocalExprs(rootdir string, cfgdir string) (exportedLocalExprs, error) {
	logger := log.With().
		Str("action", "loadStackExportedLocalExprs()").
		Str("path", rootdir).
		Logger()

	logger.Debug().Msg("Parse export as locals blocks.")

	blocks, err := hcl.ParseExportAsLocalsBlocks(filepath.Join(rootdir, cfgdir))
	if err != nil {
		return exportedLocalExprs{}, fmt.Errorf("%w: %v", ErrExportedLocalsParsing, err)
	}

	exportLocals := newExportedLocalExprs()

	logger.Trace().Msg("Range over blocks.")

	for filename, hclblocks := range blocks {
		for _, hclblock := range hclblocks {
			logger := log.With().
				Str("filename", filename).
				Logger()
			logger.Trace().Msg("Range over block attributes.")

			for name, attr := range hclblock.Body.Attributes {
				if exportLocals.has(name) {
					return exportedLocalExprs{}, fmt.Errorf(
						"%w: export_as_locals %q redefined in configuration %q",
						ErrExportedLocalRedefined,
						name,
						filepath.Join(cfgdir, filename),
					)
				}
				exportLocals.expressions[name] = attr.Expr
			}
		}
	}

	logger.Trace().Msg("Get parent config.")

	parentcfg, ok := parentDir(cfgdir)
	if !ok {
		return exportLocals, nil
	}

	logger.Debug().Msg("Get parent export locals.")

	parentExportLocals, err := loadStackExportedLocalExprs(rootdir, parentcfg)
	if err != nil {
		return exportedLocalExprs{}, err
	}

	logger.Trace().Msg("Merge export locals with parent export locals.")

	exportLocals.merge(parentExportLocals)
	return exportLocals, nil
}

type exportedLocalExprs struct {
	expressions map[string]hclsyntax.Expression
}

func (r exportedLocalExprs) merge(other exportedLocalExprs) {
	for k, v := range other.expressions {
		if !r.has(k) {
			r.expressions[k] = v
		}
	}
}

func (r exportedLocalExprs) has(name string) bool {
	_, ok := r.expressions[name]
	return ok
}

func (r exportedLocalExprs) eval(meta stack.Metadata, globals Globals) (ExportedLocalValues, error) {
	// FIXME(katcipis): get abs path for stack.
	// This is relative only to root since meta.Path will look
	// like: /some/path/relative/project/root

	logger := log.With().
		Str("action", "eval()").
		Str("stack", meta.Path).
		Logger()

	logger.Trace().
		Msg("Create new context.")
	evalctx := eval.NewContext("." + meta.Path)

	logger.Trace().
		Msg("Add proper namespace for stack metadata evaluation.")
	err := evalctx.SetNamespace("terramate", meta.ToCtyMap())
	if err != nil {
		return ExportedLocalValues{}, fmt.Errorf("evaluating export_as_locals: setting terramate metadata namespace: %v", err)
	}

	logger.Trace().
		Msg("Add proper namespace for globals evaluation.")
	err = evalctx.SetNamespace("global", globals.Attributes())
	if err != nil {
		return ExportedLocalValues{}, fmt.Errorf("evaluating export_as_locals: setting terramate globals namespace: %v", err)
	}

	var errs []error
	exportAsLocals := newExportedLocalValues()

	logger.Trace().
		Msg("Range over exported locals expressions.")
	for name, expr := range r.expressions {
		val, err := evalctx.Eval(expr)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		exportAsLocals.attributes[name] = val
	}

	// TODO(katcipis): error reporting can be improved here.
	logger.Trace().
		Msg("Reduce errors to single error.")
	err = errutil.Reduce(func(err1 error, err2 error) error {
		return fmt.Errorf("%v,%v", err1, err2)
	}, errs...)

	if err != nil {
		return ExportedLocalValues{}, fmt.Errorf("evaluating export_as_locals attributes: [%v]", err)
	}

	return exportAsLocals, nil
}

func newExportedLocalExprs() exportedLocalExprs {
	return exportedLocalExprs{
		expressions: map[string]hclsyntax.Expression{},
	}
}

func newExportedLocalValues() ExportedLocalValues {
	return ExportedLocalValues{
		attributes: map[string]cty.Value{},
	}
}
