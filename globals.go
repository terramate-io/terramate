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
	tflang "github.com/hashicorp/terraform/lang"
	"github.com/madlambda/spells/errutil"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/zclconf/go-cty/cty"
)

// Globals represents a globals block. Always use NewGlobals to create it.
type Globals struct {
	evaluated   map[string]cty.Value
	nonEvaluted map[string]expression
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

	globals, err := loadStackGlobals(rootdir, meta.Path)
	if err != nil {
		return nil, err
	}
	if err := globals.Eval(meta); err != nil {
		return nil, err
	}

	return globals, nil
}

func NewGlobals() *Globals {
	return &Globals{
		evaluated:   map[string]cty.Value{},
		nonEvaluted: map[string]expression{},
	}
}

// Iter iterates the globals. There is no order guarantee on the iteration.
func (g *Globals) Iter(iter func(name string, val cty.Value)) {
	for name, val := range g.evaluated {
		iter(name, val)
	}
}

// Equal checks if two StackGlobals are equal. They are equal if both
// have globals with the same name=value.
func (g *Globals) Equal(other *Globals) bool {
	if len(g.evaluated) != len(other.evaluated) {
		return false
	}

	for k, v := range other.evaluated {
		val, ok := g.evaluated[k]
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
		return fmt.Errorf("parsing expression %q=%q: %v", key, expr, diags)
	}
	// It is remarkably hard to write HCL from an expression:
	// https://stackoverflow.com/questions/67945463/how-to-use-hcl-write-to-set-expressions-with
	// So this ugly hack for now :-(
	// And yes, it seemed like a good idea to have two token types on each library.
	hcltokens, diags := hclsyntax.LexExpression([]byte(expr), "", hhcl.Pos{})
	if diags.HasErrors() {
		return fmt.Errorf("tokenizing expression %q=%q: %v", key, expr, diags)
	}
	tokens := make([]*hclwrite.Token, len(hcltokens))

	for i, t := range hcltokens {
		tokens[i] = &hclwrite.Token{
			Type:  t.Type,
			Bytes: t.Bytes,
		}
	}

	g.nonEvaluted[key] = expression{
		expr:   parsed,
		tokens: tokens,
	}
	return nil
}

// Eval evaluates any pending expressions on the context of a specific stack.
// It is safe to call Eval with the same metadata multiple times.
func (g *Globals) Eval(meta StackMetadata) error {

	// TODO(katcipis): add BaseDir on Scope.
	tfscope := &tflang.Scope{}
	evalctx, err := newHCLEvalContext(meta, tfscope)
	if err != nil {
		return err
	}

	for k, p := range g.nonEvaluted {
		val, err := p.expr.Value(evalctx)
		if err != nil {
			return err
		}
		g.evaluated[k] = val
	}
	g.nonEvaluted = map[string]expression{}
	return nil
}

// String representation of the stack globals as HCL.
func (g *Globals) String() string {
	gen := hclwrite.NewEmptyFile()
	rootBody := gen.Body()
	tfBlock := rootBody.AppendNewBlock("globals", nil)
	tfBody := tfBlock.Body()

	for name, val := range g.evaluated {
		tfBody.SetAttributeValue(name, val)
	}

	for name, pending := range g.nonEvaluted {
		// Not sure the best way to approach this.
		// Just want to add raw expressions back to HCL but
		// it was quite confusing with traversals and traversers, etc.
		//
		// - https://stackoverflow.com/questions/67945463/how-to-use-hcl-write-to-set-expressions-with
		tfBody.SetAttributeRaw(name, pending.tokens)
	}

	// Tokens logic for pending expressions introduces messed up formatting.
	return string(hclwrite.Format(gen.Bytes()))
}

func (g *Globals) merge(other *Globals) {
	for k, v := range other.nonEvaluted {
		_, ok := g.nonEvaluted[k]
		if !ok {
			g.nonEvaluted[k] = v
		}
	}
}

type expression struct {
	expr   hclsyntax.Expression
	tokens hclwrite.Tokens
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
			if _, ok := globals.nonEvaluted[name]; ok {
				return nil, fmt.Errorf("%w: global %q already defined", ErrGlobalRedefined, name)
			}
			// Would be consistent to also initialize
			// tokens on the pendingExpression, but betting tokens on this
			// scenario is quite non-trivial sadly and not a requirement
			// for core features since when loading globals we always
			// evaluate it before returning to the caller, so there
			// will be no pending expression on the Globals object anyway.
			// When manually building Globals then this can be an issue (see Globals.AddExpr).
			globals.nonEvaluted[name] = expression{
				expr: attr.Expr,
			}
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
