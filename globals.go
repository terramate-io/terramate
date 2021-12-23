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
	tflang "github.com/hashicorp/terraform/lang"
	"github.com/madlambda/spells/errutil"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/zclconf/go-cty/cty"
)

// Globals represents a globals block.
type Globals struct {
	attributes map[string]cty.Value
}

const ErrGlobalRedefined errutil.Error = "global redefined"

// LoadStackGlobals loads from the file system all globals defined for
// a given stack. It will navigate the file system from the stack dir until
// it reaches rootdir, loading globals and merging them appropriately.
//
// More specific globals (closer or at the stack) have precedence over
// less specific globals (closer or at the root dir).
//
// Metadata for the stack is used on the evaluation of globals, defined on stackmeta.
// The rootdir MUST be an absolute path.
func LoadStackGlobals(rootdir string, meta StackMetadata) (*Globals, error) {
	if !filepath.IsAbs(rootdir) {
		return nil, fmt.Errorf("%q is not absolute path", rootdir)
	}

	unEvalGlobals, err := loadStackGlobals(rootdir, meta.Path)
	if err != nil {
		return nil, err
	}
	return unEvalGlobals.eval(meta)
}

// Attributes returns all the global attributes, the key in the map
// is the attribute name with its corresponding value mapped
func (g *Globals) Attributes() map[string]cty.Value {
	return g.attributes
}

func (g *Globals) set(name string, val cty.Value) {
	g.attributes[name] = val
}

type rawGlobals struct {
	expressions map[string]hclsyntax.Expression
}

func (r *rawGlobals) merge(other *rawGlobals) {
	for k, v := range other.expressions {
		if !r.has(k) {
			r.add(k, v)
		}
	}
}

func (r *rawGlobals) add(name string, expr hclsyntax.Expression) {
	r.expressions[name] = expr
}

func (r *rawGlobals) has(name string) bool {
	_, ok := r.expressions[name]
	return ok
}

func (r *rawGlobals) eval(meta StackMetadata) (*Globals, error) {
	// TODO(katcipis): add BaseDir on Scope.
	tfscope := &tflang.Scope{}
	evalctx, err := newHCLEvalContext(meta, tfscope)
	if err != nil {
		return nil, err
	}

	// error messages improve if globals is empty instead of undefined
	empty, err := hclMapToCty(nil)
	if err != nil {
		return nil, fmt.Errorf("initializing empty globals: %v", err)
	}

	evalctx.Variables["global"] = empty

	var errs []error
	globals := newGlobals()
	pendingExprs := r.expressions

	for len(pendingExprs) > 0 {
		evaluated := 0

		for name, expr := range pendingExprs {
			val, err := expr.Value(evalctx)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			globals.set(name, val)
			evaluated += 1
			delete(pendingExprs, name)
		}

		if evaluated == 0 {
			break
		}

		attrs := globals.Attributes()
		globalsObj, err := hclMapToCty(attrs)

		if err != nil {
			return nil, fmt.Errorf("evaluating globals: unexpected %v", err)
		}

		evalctx.Variables["global"] = globalsObj
		errs = nil
	}

	err = errutil.Reduce(func(err1 error, err2 error) error {
		return fmt.Errorf("%v:%v", err1, err2)
	}, errs...)

	if err != nil {
		return nil, fmt.Errorf("evaluating globals: %v", err)
	}

	return globals, nil
}

func newRawGlobals() *rawGlobals {
	return &rawGlobals{
		expressions: map[string]hclsyntax.Expression{},
	}
}

func loadStackGlobals(rootdir string, cfgdir string) (*rawGlobals, error) {
	cfgpath := filepath.Join(rootdir, cfgdir, config.Filename)
	blocks, err := hcl.ParseGlobalsBlocks(cfgpath)

	if os.IsNotExist(err) {
		parentcfg, ok := parentDir(cfgdir)
		if !ok {
			return newRawGlobals(), nil
		}
		return loadStackGlobals(rootdir, parentcfg)

	}

	if err != nil {
		return nil, err
	}

	globals := newRawGlobals()

	for _, block := range blocks {
		for name, attr := range block.Body.Attributes {
			if globals.has(name) {
				return nil, fmt.Errorf("%w: global %q already defined in configuration %q", ErrGlobalRedefined, name, cfgpath)
			}
			globals.add(name, attr.Expr)
		}
	}

	parentcfg, ok := parentDir(cfgdir)
	if !ok {
		return globals, nil
	}

	parentGlobals, err := loadStackGlobals(rootdir, parentcfg)
	if err != nil {
		return nil, err
	}

	globals.merge(parentGlobals)
	return globals, nil
}

func parentDir(dir string) (string, bool) {
	parent := filepath.Dir(dir)
	return parent, parent != dir
}

func newGlobals() *Globals {
	return &Globals{
		attributes: map[string]cty.Value{},
	}
}
