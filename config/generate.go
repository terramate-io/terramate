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

package config

import (
	"path"

	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/hcl/info"
	"github.com/mineiros-io/terramate/lets"
	"github.com/mineiros-io/terramate/project"

	"github.com/zclconf/go-cty/cty"
)

const (
	// ErrInvalidConditionType indicates the condition attribute
	// has an invalid type.
	ErrInvalidConditionType errors.Kind = "invalid condition type"

	// ErrConditionEval indicates an error when evaluating the condition attribute.
	ErrConditionEval errors.Kind = "evaluating condition"

	ErrInvalidLabel errors.Kind = "invalid label"
)

// Generate represents a generate_file and generate_hcl block.
type Generate[T hcl.GenerateContent] struct {
	target    project.Path
	condition bool
	content   string
	context   string
	asserts   []Assert
	origin    info.Range
	scope     *Tree
	evalctx   *eval.Context
	Block     hcl.GenerateBlock[T]
}

func (g Generate[T]) Target() project.Path       { return g.target }
func (g Generate[T]) Condition() bool            { return g.condition }
func (g Generate[T]) Content() string            { return g.content }
func (g Generate[T]) Context() string            { return g.context }
func (g Generate[T]) Asserts() []Assert          { return g.asserts }
func (g Generate[T]) Origin() info.Range         { return g.origin }
func (g Generate[T]) Scope() *Tree               { return g.scope }
func (g Generate[T]) EvalContext() *eval.Context { return g.evalctx }
func (g Generate[T]) Header() string             { return g.Block.Content.Header() }

func EvalGenerate[T hcl.GenerateContent](
	globalctx *eval.Context,
	block hcl.GenerateBlock[T],
	scope *Tree,
) (Generate[T], error) {
	evalctx := globalctx.Copy()

	target, err := computeTarget(block, scope)
	if err != nil {
		return Generate[T]{}, err
	}

	generate := Generate[T]{
		target:  target,
		origin:  block.Range,
		context: block.Context,
		scope:   scope,
		evalctx: evalctx,
	}

	err = lets.Load(block.Lets, evalctx)
	if err != nil {
		return Generate[T]{}, err
	}

	condition := true
	if block.Condition != nil {
		value, err := evalctx.Eval(block.Condition.Expr)
		if err != nil {
			return Generate[T]{}, errors.E(ErrConditionEval, err)
		}
		if value.Type() != cty.Bool {
			return Generate[T]{}, errors.E(
				ErrInvalidConditionType,
				"condition has type %s but must be boolean",
				value.Type().FriendlyName(),
			)
		}
		condition = value.True()
	}

	generate.condition = condition
	if !condition {
		return generate, nil
	}

	asserts := make([]Assert, len(block.Asserts))
	assertsErrs := errors.L()
	assertFailed := false

	for i, assertCfg := range block.Asserts {
		assert, err := EvalAssert(evalctx, assertCfg)
		if err != nil {
			assertsErrs.Append(err)
			continue
		}
		asserts[i] = assert
		if !assert.Assertion && !assert.Warning {
			assertFailed = true
		}
	}

	if err := assertsErrs.AsError(); err != nil {
		return Generate[T]{}, err
	}

	generate.asserts = asserts
	if assertFailed {
		return generate, nil
	}

	content, err := block.Content.Body(evalctx)
	if err != nil {
		return Generate[T]{}, err
	}

	generate.content = content
	return generate, nil
}

func computeTarget[T hcl.GenerateContent](block hcl.GenerateBlock[T], scope *Tree) (project.Path, error) {
	if block.Context == "root" {
		if !path.IsAbs(block.Label) {
			return "", errors.E(
				ErrInvalidLabel,
				"generate block with context=root requires an absolute project path but given %s",
				block.Label,
			)
		}
		return project.NewPath(block.Label), nil
	}
	if path.IsAbs(block.Label) {
		return "", errors.E(
			ErrInvalidLabel,
			"generate block with context=stack (or omitted) requires a relative path but given %s",
			block.Label,
		)
	}
	return project.NewPath(path.Join(scope.ProjDir().String(), block.Label)), nil
}
