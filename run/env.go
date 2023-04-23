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

package run

import (
	"os"
	"strings"

	"github.com/mineiros-io/terramate/runtime"

	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/globals"
	"github.com/mineiros-io/terramate/hcl/ast"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/stdlib"

	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
)

const (
	// ErrEval indicates that an error happened while evaluating one of the
	// terramate.config.run.env attributes.
	ErrEval errors.Kind = "evaluating terramate.config.run.env attribute"

	// ErrInvalidEnvVarType indicates the env var attribute
	// has an invalid type.
	ErrInvalidEnvVarType errors.Kind = "invalid environment variable type"
)

// EnvVars represents a set of environment variables to be used
// when running commands. Each string follows the same format used
// on os.Environ and can be used to set env on exec.Cmd.
type EnvVars []string

// LoadEnv will load environment variables to be exported when running any command
// inside the given stack. The order of the env vars is guaranteed to be the same
// and is ordered lexicographically.
func LoadEnv(root *config.Root, st *config.Stack) (EnvVars, error) {
	logger := log.With().
		Str("action", "run.Env()").
		Str("root", root.HostDir()).
		Stringer("stack", st).
		Logger()

	logger.Trace().Msg("checking if we have run env config")

	if !root.Tree().Node.HasRunEnv() {
		logger.Trace().Msg("no run env config found, nothing to do")
		return nil, nil
	}

	logger.Trace().Msg("loading globals")

	tree, _ := root.Lookup(st.Dir)

	evalctx := eval.New(
		globals.NewResolver(tree),
		runtime.NewResolver(root, st),
		&resolver{
			env: os.Environ(),
		},
	)
	evalctx.SetFunctions(stdlib.Functions(evalctx, st.HostDir(root)))

	envVars := EnvVars{}

	attrs := root.Tree().Node.Terramate.Config.Run.Env.Attributes.SortedList()

	for _, attr := range attrs {
		logger = logger.With().
			Str("attribute", attr.Name).
			Logger()

		logger.Trace().Msg("evaluating")

		val, err := evalctx.Eval(attr.Expr)
		if err != nil {
			return nil, errors.E(ErrEval, err)
		}

		logger.Trace().Msg("checking evaluated value type")

		if val.Type() != cty.String {
			return nil, errors.E(
				ErrInvalidEnvVarType,
				attr.Range,
				"attr has type %s but must be string",
				val.Type().FriendlyName(),
			)
		}
		envVars = append(envVars, attr.Name+"="+val.AsString())

		logger.Trace().Msg("env var loaded")
	}

	return envVars, nil
}

type resolver struct {
	env []string
}

func (r *resolver) Root() string { return "env" }

func (r *resolver) Prevalue() cty.Value { return cty.EmptyObjectVal }

func (r *resolver) LoadStmts() (eval.Stmts, error) {
	stmts := make(eval.Stmts, len(r.env))
	for _, env := range r.env {
		nameval := strings.Split(env, "=")

		ref := eval.Ref{
			Object: r.Root(),
			Path:   []string{nameval[0]},
		}

		val := cty.StringVal(strings.Join(nameval[1:], "="))
		tokens := ast.TokensForValue(val)
		expr, _ := ast.ParseExpression(string(tokens.Bytes()), `<environ>`)

		stmts = append(stmts, eval.Stmt{
			Origin: ref,
			LHS:    ref,
			RHS:    expr,
			Scope:  project.NewPath("/"), // env is root-scoped
		})
	}
	return stmts, nil
}

func (r *resolver) LookupRef(ref eval.Ref) (eval.Stmts, error) {
	stmts, err := r.LoadStmts()
	if err != nil {
		return nil, err
	}
	filtered, _ := stmts.SelectBy(ref, map[eval.RefStr]eval.Ref{})
	return filtered, nil
}
