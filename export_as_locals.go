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
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/madlambda/spells/errutil"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/zclconf/go-cty/cty"
)

// ExportAsLocals represents a export_as_locals block.
type ExportAsLocals struct {
	attributes map[string]cty.Value
}

// LoadStackExportAsLocals loads from the file system all export_as_locals for
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
func LoadStackExportAsLocals(rootdir string, sm StackMetadata, g *Globals) (ExportAsLocals, error) {
	if !filepath.IsAbs(rootdir) {
		return ExportAsLocals{}, fmt.Errorf("%q must be an absolute path", rootdir)
	}

	unEvalExport, err := loadStackExportAsLocals(rootdir, sm.Path)
	if err != nil {
		return ExportAsLocals{}, err
	}
	return unEvalExport.eval(sm, g)
}

func (e ExportAsLocals) Attributes() map[string]cty.Value {
	attrcopy := map[string]cty.Value{}
	for k, v := range e.attributes {
		attrcopy[k] = v
	}
	return attrcopy
}

func loadStackExportAsLocals(rootdir string, cfgdir string) (unEvalExportAsLocals, error) {
	cfgpath := filepath.Join(rootdir, cfgdir, config.Filename)
	blocks, err := hcl.ParseExportAsLocalsBlocks(cfgpath)

	if os.IsNotExist(err) {
		parentcfg, ok := parentDir(cfgdir)
		if !ok {
			return newUnEvalExportAsLocals(), nil
		}
		return loadStackExportAsLocals(rootdir, parentcfg)
	}

	if err != nil {
		return unEvalExportAsLocals{}, err
	}

	exportLocals := newUnEvalExportAsLocals()

	for _, block := range blocks {
		for name, attr := range block.Body.Attributes {
			// TODO(katcipis): test behavior
			//if globals.has(name) {
			//return nil, fmt.Errorf("%w: global %q already defined in configuration %q", ErrGlobalRedefined, name, cfgpath)
			//}
			exportLocals.add(name, attr.Expr)
		}
	}

	parentcfg, ok := parentDir(cfgdir)
	if !ok {
		return exportLocals, nil
	}

	parentExportLocals, err := loadStackExportAsLocals(rootdir, parentcfg)
	if err != nil {
		return unEvalExportAsLocals{}, err
	}

	exportLocals.merge(parentExportLocals)
	return exportLocals, nil
}

type unEvalExportAsLocals struct {
	expressions map[string]hclsyntax.Expression
}

func (r unEvalExportAsLocals) merge(other unEvalExportAsLocals) {
	for k, v := range other.expressions {
		if !r.has(k) {
			r.add(k, v)
		}
	}
}

func (r unEvalExportAsLocals) add(name string, expr hclsyntax.Expression) {
	r.expressions[name] = expr
}

func (r unEvalExportAsLocals) has(name string) bool {
	_, ok := r.expressions[name]
	return ok
}

func (r unEvalExportAsLocals) eval(meta StackMetadata, globals *Globals) (ExportAsLocals, error) {
	// FIXME(katcipis): get abs path for stack.
	// This is relative only to root since meta.Path will look
	// like: /some/path/relative/project/root
	evalctx := eval.NewContext("." + meta.Path)

	// TODO(katcipis): test behavior
	//if err := meta.SetOnEvalCtx(evalctx); err != nil {
	//return nil, fmt.Errorf("evaluating export_as_locals: setting terramate metadata namespace: %v", err)
	//}

	if err := globals.SetOnEvalCtx(evalctx); err != nil {
		return ExportAsLocals{}, fmt.Errorf("evaluating export_as_locals: setting terramate globals namespace: %v", err)
	}

	var errs []error
	exportAsLocals := newExportAsLocals()

	for name, expr := range r.expressions {
		val, err := evalctx.Eval(expr)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		exportAsLocals.attributes[name] = val
	}

	// TODO(katcipis): error reporting can be improved here.
	err := errutil.Reduce(func(err1 error, err2 error) error {
		return fmt.Errorf("%v,%v", err1, err2)
	}, errs...)

	if err != nil {
		return ExportAsLocals{}, fmt.Errorf("evaluating export_as_locals attributes: [%v]", err)
	}

	return exportAsLocals, nil
}

func newUnEvalExportAsLocals() unEvalExportAsLocals {
	return unEvalExportAsLocals{
		expressions: map[string]hclsyntax.Expression{},
	}
}

func newExportAsLocals() ExportAsLocals {
	return ExportAsLocals{
		attributes: map[string]cty.Value{},
	}
}
