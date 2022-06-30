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

package stack

import (
	"path/filepath"

	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/project"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
)

// Globals represents information obtained by parsing and evaluating globals blocks.
type Globals struct {
	attributes map[string]cty.Value
}

// Errors returned when parsing and evaluating globals.
const (
	ErrGlobalEval      errors.Kind = "globals eval failed"
	ErrGlobalRedefined errors.Kind = "global redefined"
)

// LoadGlobals loads from the file system all globals defined for
// a given stack. It will navigate the file system from the stack dir until
// it reaches rootdir, loading globals and merging them appropriately.
//
// More specific globals (closer or at the stack) have precedence over
// less specific globals (closer or at the root dir).
//
// Metadata for the stack is used on the evaluation of globals.
// The rootdir MUST be an absolute path.
func LoadGlobals(rootdir string, meta Metadata) (Globals, error) {
	logger := log.With().
		Str("action", "LoadStackGlobals()").
		Str("stack", meta.Path()).
		Logger()

	if !filepath.IsAbs(rootdir) {
		return Globals{}, errors.E("%q is not absolute path", rootdir)
	}

	logger.Debug().Msg("Load stack globals.")

	globalsExprs, err := loadStackGlobalsExprs(rootdir, meta.Path())
	if err != nil {
		return Globals{}, err
	}
	return globalsExprs.eval(rootdir, meta)
}

// Attributes returns all the global attributes, the key in the map
// is the attribute name with its corresponding value mapped
func (g Globals) Attributes() map[string]cty.Value {
	attrcopy := map[string]cty.Value{}
	for k, v := range g.attributes {
		attrcopy[k] = v
	}
	return attrcopy
}

// String provides a string representation of the globals
func (g Globals) String() string {
	return hcl.FormatAttributes(g.attributes)
}

type expression struct {
	origin string
	value  hclsyntax.Expression
}

type globalsExpr struct {
	expressions map[string]expression
}

func (ge *globalsExpr) merge(other *globalsExpr) {
	for k, v := range other.expressions {
		if !ge.has(k) {
			ge.add(k, v)
		}
	}
}

func (ge *globalsExpr) add(name string, expr expression) {
	ge.expressions[name] = expr
}

func (ge *globalsExpr) has(name string) bool {
	_, ok := ge.expressions[name]
	return ok
}

func (ge *globalsExpr) eval(rootdir string, meta Metadata) (Globals, error) {
	// FIXME(katcipis): get abs path for stack.
	// This is relative only to root since meta.Path will look
	// like: /some/path/relative/project/root
	logger := log.With().
		Str("action", "globals.eval()").
		Str("stack", meta.Path()).
		Logger()

	logger.Trace().Msg("Create new evaluation context.")

	globals := Globals{
		attributes: map[string]cty.Value{},
	}
	evalctx := NewEvalCtx(rootdir, meta, globals)

	pendingExprsErrs := map[string]error{}
	pendingExprs := ge.expressions

	for len(pendingExprs) > 0 {
		amountEvaluated := 0

		logger.Trace().Msg("evaluating pending expressions")

	pendingExpression:
		for name, expr := range pendingExprs {
			logger := logger.With().
				Str("origin", expr.origin).
				Str("global", name).
				Logger()

			vars := hclsyntax.Variables(expr.value)

			logger.Trace().Msg("checking var access inside expression")

			for _, namespace := range vars {
				if !evalctx.HasNamespace(namespace.RootName()) {
					return Globals{}, errors.E(
						ErrGlobalEval,
						namespace.SourceRange(),
						"unknown variable namespace: %s", namespace.RootName(),
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
				default:
					panic("unexpected type of traversal - this is a BUG")
				}
			}

			logger.Trace().Msg("evaluating expression")

			val, err := evalctx.Eval(expr.value)
			if err != nil {
				pendingExprsErrs[name] = err
				continue
			}

			globals.attributes[name] = val
			amountEvaluated++

			delete(pendingExprs, name)
			delete(pendingExprsErrs, name)

			logger.Trace().Msg("updating globals eval context with evaluated attribute")

			evalctx.SetGlobals(globals)
		}

		if amountEvaluated == 0 {
			break
		}
	}

	if len(pendingExprs) > 0 {
		// TODO(katcipis): model proper error list and return that
		// Caller can decide how to format/log things (like code generation report).
		for name, expr := range pendingExprs {
			err, ok := pendingExprsErrs[name]
			if !ok {
				err = errors.E("undefined global")
			}
			logger.Err(err).
				Str("name", name).
				Str("origin", expr.origin).
				Msg("evaluating global")
		}
		return Globals{}, errors.E(
			ErrGlobalEval,
			"unable to evaluate %d globals", len(pendingExprs),
		)
	}

	return globals, nil
}

func newGlobalsExpr() *globalsExpr {
	return &globalsExpr{
		expressions: map[string]expression{},
	}
}

func loadStackGlobalsExprs(rootdir string, cfgdir string) (*globalsExpr, error) {
	logger := log.With().
		Str("action", "loadStackGlobalsExpr()").
		Str("root", rootdir).
		Str("cfgdir", cfgdir).
		Logger()

	globals := newGlobalsExpr()

	logger.Debug().Msg("Parse globals blocks.")

	absdir := filepath.Join(rootdir, cfgdir)
	p := hcl.NewTerramateParser(absdir)
	err := p.AddDir(absdir)
	if err != nil {
		return nil, errors.E("adding dir to parser", err)
	}

	err = p.MinimalParse()
	if err != nil {
		return nil, errors.E("parsing config", err)
	}

	globalsBlock, ok := p.MergedBlocks["globals"]
	if ok {
		logger.Trace().Msg("Range over attributes.")

		for _, attr := range globalsBlock.Attributes.SortedList() {
			logger.Trace().Msg("Add attribute to globals.")

			globals.add(attr.Name, expression{
				origin: project.PrjAbsPath(rootdir, attr.Origin),
				value:  attr.Expr,
			})
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
