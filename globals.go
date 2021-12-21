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

	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/madlambda/spells/errutil"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/zclconf/go-cty/cty"
)

// Globals represents a globals block. Always use NewGlobals to create it.
type Globals struct {
	data         map[string]cty.Value
	pendingExprs map[string]hclsyntax.Expression
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
func LoadStackGlobals(rootdir string, stackmeta StackMetadata) (*Globals, error) {
	globals, err := loadStackGlobals(rootdir, stackmeta.Path)
	if err != nil {
		return nil, err
	}
	if err := globals.Eval(); err != nil {
		return nil, err
	}
	return globals, nil
}

func NewGlobals() *Globals {
	return &Globals{
		data:         map[string]cty.Value{},
		pendingExprs: map[string]hclsyntax.Expression{},
	}
}

// Equal checks if two StackGlobals are equal. They are equal if both
// have globals with the same name=value.
func (g *Globals) Equal(other *Globals) bool {
	if len(g.data) != len(other.data) {
		return false
	}

	for k, v := range other.data {
		val, ok := g.data[k]
		if !ok {
			return false
		}
		if !v.RawEquals(val) {
			return false
		}
	}

	return true
}

// AddExpr adds a new expression to the global.
// It will not be evaluated until Eval is called.
// Returns an error if the expression is not a valid HCL expression.
func (g *Globals) AddExpr(key string, expr string) error {
	parsed, diags := hclsyntax.ParseExpression([]byte(expr), "", hhcl.Pos{})
	if diags.HasErrors() {
		return fmt.Errorf("adding %q=%q: %v", key, expr, diags)
	}
	g.pendingExprs[key] = parsed
	return nil
}

// Eval evaluates any pending expressions.
// It can be called multiple times, if there are no new expressions
// since the last Eval call it won't do anything.
func (g *Globals) Eval() error {
	for k, expr := range g.pendingExprs {
		val, _ := expr.Value(nil)
		g.data[k] = val
	}
	g.pendingExprs = map[string]hclsyntax.Expression{}
	return nil
}

// String representation of the stack globals as HCL.
// It is an error to call this if Eval hasn't been successfully called
// yet and will create a string representation that is not valid HCL.
func (g *Globals) String() string {
	if len(g.pendingExprs) > 0 {
		return fmt.Sprintf("has unevaluated expressions: %v", g.pendingExprs)
	}

	gen := hclwrite.NewEmptyFile()
	rootBody := gen.Body()
	tfBlock := rootBody.AppendNewBlock("globals", nil)
	tfBody := tfBlock.Body()

	for name, val := range g.data {
		tfBody.SetAttributeValue(name, val)
	}

	return string(gen.Bytes())
}

func (g *Globals) merge(other *Globals) {
	for k, v := range other.pendingExprs {
		_, ok := g.pendingExprs[k]
		if !ok {
			g.pendingExprs[k] = v
		}
	}
}

func loadStackGlobals(rootdir string, cfgdir string) (*Globals, error) {
	cfgpath := filepath.Join(rootdir, cfgdir, config.Filename)
	blocks, err := hcl.ParseGlobalsBlocks(cfgpath)

	if os.IsNotExist(err) {
		parentcfg, ok := parentDir(cfgdir)
		if !ok {
			return NewGlobals(), nil
		}
		return loadStackGlobals(rootdir, parentcfg)

	}

	if err != nil {
		return nil, err
	}

	globals := NewGlobals()

	for _, block := range blocks {
		for name, attr := range block.Body.Attributes {
			if _, ok := globals.pendingExprs[name]; ok {
				return nil, fmt.Errorf("%w: global %q already defined", ErrGlobalRedefined, name)
			}
			globals.pendingExprs[name] = attr.Expr
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
