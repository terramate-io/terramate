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

	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/stack"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
)

const (
	// ErrParsingCfg indicates that an error happened while parsing configuration.
	ErrParsingCfg errors.Kind = "parsing terramate.config.run.env configuration"

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
func LoadEnv(rootdir string, st *stack.S) (EnvVars, error) {
	logger := log.With().
		Str("action", "run.Env()").
		Str("root", rootdir).
		Stringer("stack", st).
		Logger()

	logger.Trace().Msg("parsing configuration")

	cfg, err := hcl.ParseDir(rootdir, rootdir)
	if err != nil {
		return nil, errors.E(ErrParsingCfg, err)
	}

	logger.Trace().Msg("checking if we have run env config")

	if !cfg.HasRunEnv() {
		logger.Trace().Msg("no run env config found, nothing to do")
		return nil, nil
	}

	logger.Trace().Msg("loading globals")

	globals, err := stack.LoadGlobals(rootdir, st)
	if err != nil {
		return nil, errors.E(ErrLoadingGlobals, err)
	}

	evalctx := stack.NewEvalCtx(rootdir, st, globals)

	evalctx.SetEnv(os.Environ())

	envVars := EnvVars{}

	attrs := cfg.Terramate.Config.Run.Env.Attributes.SortedList()

	for _, attr := range attrs {
		logger = logger.With().
			Str("attribute", attr.Name).
			Logger()

		logger.Trace().Msg("evaluating")

		val, err := evalctx.Eval(attr.Expr)
		if err != nil {
			return nil, errors.E(ErrEval, attr.NameRange,
				err, "attribute origin %s", attr.Origin)
		}

		logger.Trace().Msg("checking evaluated value type")

		if val.Type() != cty.String {
			return nil, errors.E(
				ErrInvalidEnvVarType,
				attr.NameRange,
				"attr has type %s but must be string, attribute origin %s",
				val.Type().FriendlyName(),
				attr.Origin,
			)
		}
		envVars = append(envVars, attr.Name+"="+val.AsString())

		logger.Trace().Msg("env var loaded")
	}

	return envVars, nil
}
