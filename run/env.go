// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package run

import (
	"os"
	"sort"
	"strings"

	"github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/globals"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/stdlib"
	"golang.org/x/exp/maps"

	"github.com/zclconf/go-cty/cty"
)

const (
	// ErrLoadingGlobals indicates that an error happened while loading globals.
	ErrLoadingGlobals errors.Kind = "loading globals to evaluate terramate.config.run.env configuration"

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
	globalsReport := globals.ForStack(root, st)
	if err := globalsReport.AsError(); err != nil {
		return nil, errors.E(ErrLoadingGlobals, err)
	}

	evalctx := eval.NewContext(stdlib.Functions(st.HostDir(root), root.Tree().Node.Experiments()))
	runtime := root.Runtime()
	runtime.Merge(st.RuntimeValues(root))
	evalctx.SetNamespace("terramate", runtime)
	evalctx.SetNamespace("global", globalsReport.Globals.AsValueMap())
	evalctx.SetEnv(os.Environ())

	tree, _ := root.Lookup(st.Dir)
	envMap := map[string]string{}
	skipMap := map[string]struct{}{}

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

// Getenv returns the value of the environment variable named by the key,
// which is assumed to be case-insensitive, from the given environment.
// If the variable is not present in the environment, found is false.
func Getenv(key string, environ []string) (val string, found bool) {
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
