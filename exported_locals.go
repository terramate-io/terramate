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

const ErrExportedLocalRedefined errutil.Error = "export_as_locals attribute redefined"

// ExportedLocalValues represents exported local values, which is information exported
// from Terramate in a way that is suitable to be used for Terraform.
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
func LoadStackExportedLocals(rootdir string, sm StackMetadata, g *Globals) (ExportedLocalValues, error) {
	if !filepath.IsAbs(rootdir) {
		return ExportedLocalValues{}, fmt.Errorf("%q must be an absolute path", rootdir)
	}

	localVars, err := loadStackExportedLocals(rootdir, sm.Path)
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

func loadStackExportedLocals(rootdir string, cfgdir string) (exportedLocalExpr, error) {
	cfgpath := filepath.Join(rootdir, cfgdir, config.Filename)
	blocks, err := hcl.ParseExportAsLocalsBlocks(cfgpath)

	if os.IsNotExist(err) {
		parentcfg, ok := parentDir(cfgdir)
		if !ok {
			return newExportedLocalExpr(), nil
		}
		return loadStackExportedLocals(rootdir, parentcfg)
	}

	if err != nil {
		return exportedLocalExpr{}, err
	}

	exportLocals := newExportedLocalExpr()

	for _, block := range blocks {
		for name, attr := range block.Body.Attributes {
			if exportLocals.has(name) {
				return exportedLocalExpr{}, fmt.Errorf(
					"%w: export_as_locals %q already defined in configuration %q",
					ErrExportedLocalRedefined,
					name,
					cfgpath,
				)
			}
			exportLocals.expressions[name] = attr.Expr
		}
	}

	parentcfg, ok := parentDir(cfgdir)
	if !ok {
		return exportLocals, nil
	}

	parentExportLocals, err := loadStackExportedLocals(rootdir, parentcfg)
	if err != nil {
		return exportedLocalExpr{}, err
	}

	exportLocals.merge(parentExportLocals)
	return exportLocals, nil
}

type exportedLocalExpr struct {
	expressions map[string]hclsyntax.Expression
}

func (r exportedLocalExpr) merge(other exportedLocalExpr) {
	for k, v := range other.expressions {
		if !r.has(k) {
			r.expressions[k] = v
		}
	}
}

func (r exportedLocalExpr) has(name string) bool {
	_, ok := r.expressions[name]
	return ok
}

func (r exportedLocalExpr) eval(meta StackMetadata, globals *Globals) (ExportedLocalValues, error) {
	// FIXME(katcipis): get abs path for stack.
	// This is relative only to root since meta.Path will look
	// like: /some/path/relative/project/root
	evalctx := eval.NewContext("." + meta.Path)

	if err := meta.SetOnEvalCtx(evalctx); err != nil {
		return ExportedLocalValues{}, fmt.Errorf("evaluating export_as_locals: setting terramate metadata namespace: %v", err)
	}

	if err := globals.SetOnEvalCtx(evalctx); err != nil {
		return ExportedLocalValues{}, fmt.Errorf("evaluating export_as_locals: setting terramate globals namespace: %v", err)
	}

	var errs []error
	exportAsLocals := newExportedLocalValues()

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
		return ExportedLocalValues{}, fmt.Errorf("evaluating export_as_locals attributes: [%v]", err)
	}

	return exportAsLocals, nil
}

func newExportedLocalExpr() exportedLocalExpr {
	return exportedLocalExpr{
		expressions: map[string]hclsyntax.Expression{},
	}
}

func newExportedLocalValues() ExportedLocalValues {
	return ExportedLocalValues{
		attributes: map[string]cty.Value{},
	}
}
