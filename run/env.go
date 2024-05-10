// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package run

import (
	"os"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/terramate-io/terramate/runtime"

	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/globals"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/hcl/info"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/stdlib"
	"golang.org/x/exp/maps"

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
// All defined `terramate.config.run.env` definitions from the provided stack dir
// up to the root of the project are collected, and env definitions closer to the
// stack have precedence over parent definitions.
func LoadEnv(root *config.Root, st *config.Stack) (EnvVars, error) {
	evalctx := eval.New(
		st.Dir,
		globals.NewResolver(root),
		runtime.NewResolver(root, st),
		&EnvResolver{
			rootdir: root.HostDir(),
			env:     os.Environ(),
		},
	)
	evalctx.SetFunctions(stdlib.Functions(evalctx, st.HostDir(root)))
	envMap := map[string]string{}
	skipMap := map[string]struct{}{}

	tree, _ := root.Lookup(st.Dir)

	for {
		if tree.Node.HasRunEnv() {
			attrs := tree.Node.Terramate.Config.Run.Env.Attributes.SortedList()

			for _, attr := range attrs {
				if _, skip := skipMap[attr.Name]; skip {
					continue
				}
				traversal, diags := hcl.AbsTraversalForExpr(attr.Expr)
				skip := !diags.HasErrors() && len(traversal) == 1 && (traversal.RootName() == "unset")
				if skip {
					skipMap[attr.Name] = struct{}{}
					continue
				}
				val, err := evalctx.Eval(attr.Expr)
				if err != nil {
					return nil, errors.E(ErrEval, err)
				}

				if val.IsNull() {
					skipMap[attr.Name] = struct{}{}
					continue
				}

				if val.Type() != cty.String {
					return nil, errors.E(
						ErrInvalidEnvVarType,
						attr.Range,
						"attr has type %s but must be string",
						val.Type().FriendlyName(),
					)
				}

				if _, ok := envMap[attr.Name]; !ok {
					envMap[attr.Name] = val.AsString()
				}
			}
		}

		tree = tree.Parent
		if tree == nil {
			break
		}
	}

	var envVars EnvVars
	keys := maps.Keys(envMap)
	sort.Strings(keys)
	for _, k := range keys {
		envVars = append(envVars, k+"="+envMap[k])
	}
	return envVars, nil
}

type EnvResolver struct {
	rootdir  string
	scopeDir project.Path
	env      []string
}

func (r *EnvResolver) Name() string { return "env" }

func (r *EnvResolver) Prevalue() cty.Value { return cty.EmptyObjectVal }

func (r *EnvResolver) loadStmts() (eval.Stmts, error) {
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

func (r *EnvResolver) LookupRef(_ project.Path, ref eval.Ref) ([]eval.Stmts, error) {
	stmts, err := r.loadStmts()
	if err != nil {
		return nil, err
	}
	filtered, _ := stmts.SelectBy(ref, map[eval.RefStr]eval.Ref{})
	return []eval.Stmts{filtered}, nil
}

func getEnv(key string, environ []string) (string, bool) {
	for i := len(environ) - 1; i >= 0; i-- {
		env := environ[i]
		for j := 0; j < len(env); j++ {
			if env[j] == '=' {
				k := env[0:j]
				if strings.EqualFold(k, key) {
					return env[j+1:], true
				}
			}
		}
	}
	return "", false
}
