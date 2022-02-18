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

	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/madlambda/spells/errutil"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/stack"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
)

// Globals represents information obtained by parsing and evaluating globals blocks.
type Globals struct {
	attributes map[string]cty.Value
}

const (
	ErrGlobalEval      errutil.Error = "globals eval failed"
	ErrGlobalParse     errutil.Error = "globals parsing failed"
	ErrGlobalRedefined errutil.Error = "global redefined"
)

// LoadStackGlobals loads from the file system all globals defined for
// a given stack. It will navigate the file system from the stack dir until
// it reaches rootdir, loading globals and merging them appropriately.
//
// More specific globals (closer or at the stack) have precedence over
// less specific globals (closer or at the root dir).
//
// Metadata for the stack is used on the evaluation of globals, defined on stackmeta.
// The rootdir MUST be an absolute path.
func LoadStackGlobals(rootdir string, meta stack.Metadata) (*Globals, error) {
	logger := log.With().
		Str("action", "LoadStackGlobals()").
		Str("stack", meta.Path).
		Logger()

	if !filepath.IsAbs(rootdir) {
		return nil, fmt.Errorf("%q is not absolute path", rootdir)
	}

	logger.Debug().Msg("Load stack globals.")

	globalsExprs, err := loadStackGlobalsExprs(rootdir, meta.Path)
	if err != nil {
		return nil, err
	}
	return globalsExprs.eval(meta)
}

// Attributes returns all the global attributes, the key in the map
// is the attribute name with its corresponding value mapped
func (g *Globals) Attributes() map[string]cty.Value {
	attrcopy := map[string]cty.Value{}
	for k, v := range g.attributes {
		attrcopy[k] = v
	}
	return attrcopy
}

// String provides a string representation of the globals
func (g *Globals) String() string {
	return hcl.FormatAttributes(g.Attributes())
}

type globalsExpr struct {
	expressions map[string]hclsyntax.Expression
}

func (r *globalsExpr) merge(other *globalsExpr) {
	for k, v := range other.expressions {
		if !r.has(k) {
			r.add(k, v)
		}
	}
}

func (r *globalsExpr) add(name string, expr hclsyntax.Expression) {
	r.expressions[name] = expr
}

func (r *globalsExpr) has(name string) bool {
	_, ok := r.expressions[name]
	return ok
}

func (r *globalsExpr) eval(meta stack.Metadata) (*Globals, error) {
	// FIXME(katcipis): get abs path for stack.
	// This is relative only to root since meta.Path will look
	// like: /some/path/relative/project/root
	logger := log.With().
		Str("action", "eval()").
		Str("stack", meta.Path).
		Logger()

	logger.Trace().Msg("Create new evaluation context.")

	evalctx := eval.NewContext("." + meta.Path)

	logger.Trace().Msg("Add proper name space for stack metadata evaluation.")

	if err := evalctx.SetNamespace("terramate", meta.ToCtyMap()); err != nil {
		return nil, err
	}

	logger.Trace().Msg("Add proper name space for globals evaluation.")

	// error messages improve if globals is empty instead of undefined
	globals := newGlobals()
	if err := evalctx.SetNamespace("global", globals.Attributes()); err != nil {
		return nil, fmt.Errorf("initializing global eval: %v", err)
	}

	var errs []error
	pendingExprs := r.expressions

	hclctx := evalctx.GetHCLContext()

	for len(pendingExprs) > 0 {
		amountEvaluated := 0

		logger.Trace().Msg("Range pending expressions.")

	pendingExpression:
		for name, expr := range pendingExprs {
			vars := hclsyntax.Variables(expr)

			logger.Trace().
				Msg("Range vars.")
			for _, namespace := range vars {
				if _, ok := hclctx.Variables[namespace.RootName()]; !ok {
					return nil, fmt.Errorf(
						"%w: unknown variable namespace: %s - %s",
						ErrGlobalEval,
						namespace.RootName(),
						namespace.SourceRange(),
					)
				}

				if namespace.RootName() != "global" {
					continue
				}

				switch attr := namespace[1].(type) {
				case hhcl.TraverseAttr:
					if _, isPending := pendingExprs[attr.Name]; isPending {
						continue pendingExpression
					}

					if _, isEvaluated := globals.attributes[attr.Name]; !isEvaluated {
						return nil, fmt.Errorf(
							"%w: unknown variable %s.%s - %s",
							ErrGlobalEval,
							namespace.RootName(),
							attr.Name,
							attr.SourceRange(),
						)
					}
				default:
					return nil, fmt.Errorf("unexpected type of traversal in %s - this is a BUG", attr.SourceRange())
				}
			}

			logger.Trace().Msg("Evaluate expression.")

			val, err := evalctx.Eval(expr)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			globals.attributes[name] = val
			amountEvaluated += 1

			logger.Trace().Msg("Delete pending expression.")

			delete(pendingExprs, name)

			logger.Trace().Msg("Try add proper namespace for globals evaluation context.")

			if err := evalctx.SetNamespace("global", globals.Attributes()); err != nil {
				return nil, fmt.Errorf("evaluating globals: %v", err)
			}
		}

		if amountEvaluated == 0 {
			break
		}

		errs = nil
	}

	if len(pendingExprs) > 0 {
		// TODO/FIX: print list of unresolved variables
		return nil, fmt.Errorf("%w: could not resolve all globals", ErrGlobalEval)
	}

	logger.Trace().Msg("Reduce multiple errors into one.")

	err := errutil.Reduce(func(err1 error, err2 error) error {
		return fmt.Errorf("%v,%v", err1, err2)
	}, errs...)

	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGlobalEval, err)
	}

	return globals, nil
}

func newGlobalsExpr() *globalsExpr {
	return &globalsExpr{
		expressions: map[string]hclsyntax.Expression{},
	}
}

func loadStackGlobalsExprs(rootdir string, cfgdir string) (*globalsExpr, error) {
	logger := log.With().
		Str("action", "loadStackGlobalsExpr()").
		Str("root", rootdir).
		Str("cfgdir", cfgdir).
		Logger()

	logger.Debug().Msg("Parse globals blocks.")

	blocks, err := hcl.ParseGlobalsBlocks(filepath.Join(rootdir, cfgdir))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGlobalParse, err)
	}

	globals := newGlobalsExpr()

	logger.Trace().Msg("Range over blocks.")

	for filename, fileblocks := range blocks {
		logger.Trace().Msg("Range over block attributes.")

		for _, fileblock := range fileblocks {
			for name, attr := range fileblock.Body.Attributes {
				if globals.has(name) {
					return nil, fmt.Errorf("%w: %q redefined in %q", ErrGlobalRedefined, name, filename)
				}

				logger.Trace().Msg("Add attribute to globals.")

				globals.add(name, attr.Expr)
			}
		}
	}

	parentcfg, ok := parentDir(cfgdir)
	if !ok {
		return globals, nil
	}

	logger.Trace().Msg("Loading stack globals from parent dir.")

	parentGlobals, err := loadStackGlobalsExprs(rootdir, parentcfg)
	if err != nil {
		return nil, err
	}

	logger.Trace().Msg("Merging globals with parent.")

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
