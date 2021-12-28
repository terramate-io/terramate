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

// SetOnEvalCtx will add the proper namespace for evaluation of globals
// on the given evaluation context.
func (g *Globals) SetOnEvalCtx(evalctx *eval.Context) error {
	return evalctx.SetNamespace("global", g.Attributes())
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
	// FIXME(katcipis): get abs path for stack.
	// This is relative only to root since meta.Path will look
	// like: /some/path/relative/project/root
	evalctx := eval.NewContext("." + meta.Path)

	if err := meta.SetOnEvalCtx(evalctx); err != nil {
		return nil, err
	}

	globals := newGlobals()
	// error messages improve if globals is empty instead of undefined
	if err := globals.SetOnEvalCtx(evalctx); err != nil {
		return nil, fmt.Errorf("initializing global eval: %v", err)
	}

	var errs []error
	pendingExprs := r.expressions

	for len(pendingExprs) > 0 {
		amountEvaluated := 0

		for name, expr := range pendingExprs {
			val, err := evalctx.Eval(expr)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			globals.set(name, val)
			amountEvaluated += 1
			delete(pendingExprs, name)
		}

		if amountEvaluated == 0 {
			break
		}

		if err := globals.SetOnEvalCtx(evalctx); err != nil {
			return nil, fmt.Errorf("evaluating globals: %v", err)
		}

		errs = nil
	}

	err := errutil.Reduce(func(err1 error, err2 error) error {
		return fmt.Errorf("%v,%v", err1, err2)
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
