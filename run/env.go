// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package run

import (
	"os"
	"strings"

	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/terramate-io/terramate/runtime"

	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/globals"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/hcl/info"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/stdlib"

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
		tree.Dir(),
		globals.NewResolver(root),
		runtime.NewResolver(root, st),
		&resolver{
			rootdir: root.HostDir(),
			env:     os.Environ(),
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
	rootdir string
	env     []string
}

func (r *resolver) Name() string { return "env" }

func (r *resolver) Prevalue() cty.Value { return cty.EmptyObjectVal }

func (r *resolver) loadStmts() (eval.Stmts, error) {
	stmts := make(eval.Stmts, len(r.env))
	for _, env := range r.env {
		nameval := strings.Split(env, "=")

		ref := eval.Ref{
			Object: r.Name(),
			Path:   []string{nameval[0]},
		}

		val := cty.StringVal(strings.Join(nameval[1:], "="))
		stmts = append(stmts, eval.Stmt{
			Origin: ref,
			LHS:    ref,
			RHS:    eval.NewValRHS(val),
			Info: eval.NewInfo(project.NewPath("/"), info.NewRange(r.rootdir, hhcl.Range{
				Start:    hhcl.InitialPos,
				End:      hhcl.InitialPos,
				Filename: `<environ>`,
			})), // env is root-scoped
		})
	}
	return stmts, nil
}

func (r *resolver) LookupRef(scope project.Path, ref eval.Ref) ([]eval.Stmts, error) {
	stmts, err := r.loadStmts()
	if err != nil {
		return nil, err
	}
	filtered, _ := stmts.SelectBy(ref, map[eval.RefStr]eval.Ref{})
	return []eval.Stmts{filtered}, nil
}
