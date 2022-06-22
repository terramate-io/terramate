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
	// ErrInvalidEnvVarType indicates the env var attribute
	// has an invalid type.
	ErrInvalidEnvVarType errors.Kind = "invalid env var type"
)

// EnvVars represents a set of environment variables to be used
// when running commands
type EnvVars map[string]string

// Env will load environment variables to be exported when running any command
// inside the given stack.
func Env(rootdir string, st stack.S) (EnvVars, error) {
	logger := log.With().
		Str("action", "run.Env()").
		Str("root", rootdir).
		Stringer("stack", st).
		Logger()
	// TODO(katcipis): test parse error handling
	logger.Trace().Msg("parsing configuration")

	cfg, _ := hcl.ParseDir(rootdir)

	if !cfg.HasRunEnv() {
		return nil, nil
	}

	// TODO(katcipis): test global load error handling
	globals, err := stack.LoadGlobals(rootdir, st)
	if err != nil {
		return nil, err
	}

	evalctx := stack.NewEvalCtx(st, globals)
	evalctx.SetEval(os.Environ())

	envVars := EnvVars{}

	for _, attribute := range cfg.Terramate.Config.Run.Env.Attributes {
		// TODO(katcipis): test eval failure error handling
		val, err := evalctx.Eval(attribute.Value().Expr)
		if err != nil {
			return nil, err
		}

		// TODO(katcipis): test eval failure wrong type
		if val.Type() != cty.String {
			return nil, errors.E(
				ErrInvalidEnvVarType,
				"env var has type %s but must be string",
				val.Type().FriendlyName(),
			)
		}
		envVars[attribute.Value().Name] = val.AsString()
	}

	return envVars, nil
}
