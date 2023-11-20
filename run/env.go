// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package run

import (
	"os"
	"strings"

	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/globals"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/stdlib"

	"github.com/rs/zerolog/log"
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
func LoadEnv(root *config.Root, st *config.Stack) (EnvVars, error) {
	logger := log.With().
		Str("action", "run.Env()").
		Str("root", root.HostDir()).
		Stringer("stack", st).
		Logger()

	if !root.Tree().Node.HasRunEnv() {
		return nil, nil
	}

	globalsReport := globals.ForStack(root, st)
	if err := globalsReport.AsError(); err != nil {
		return nil, errors.E(ErrLoadingGlobals, err)
	}

	evalctx := eval.NewContext(stdlib.Functions(st.HostDir(root)))
	runtime := root.Runtime()
	runtime.Merge(st.RuntimeValues(root))
	evalctx.SetNamespace("terramate", runtime)
	evalctx.SetNamespace("global", globalsReport.Globals.AsValueMap())
	evalctx.SetEnv(os.Environ())

	envVars := EnvVars{}

	attrs := root.Tree().Node.Terramate.Config.Run.Env.Attributes.SortedList()

	for _, attr := range attrs {
		logger = logger.With().
			Str("attribute", attr.Name).
			Logger()

		val, err := evalctx.Eval(attr.Expr)
		if err != nil {
			return nil, errors.E(ErrEval, err)
		}

		if val.Type() != cty.String {
			return nil, errors.E(
				ErrInvalidEnvVarType,
				attr.Range,
				"attr has type %s but must be string",
				val.Type().FriendlyName(),
			)
		}
		envVars = append(envVars, attr.Name+"="+val.AsString())

	}

	return envVars, nil
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
