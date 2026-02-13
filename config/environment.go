// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package config

import (
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/hcl/info"
)

// Environment is the evaluated environment block.
type Environment struct {
	ID          string
	Name        string
	Description string
	PromoteFrom string
	Info        info.Range
}

// EvalEnvironments evaluates the environment blocks from the root configuration.
func EvalEnvironments(root *Root, evalctx *eval.Context) ([]*Environment, error) {
	cfg := root.Tree().Node

	result := make([]*Environment, 0, len(cfg.Environments))
	seenEnvs := make(map[string]*Environment, len(cfg.Environments))

	for _, envHCL := range cfg.Environments {
		env, err := evalEnvironment(evalctx, envHCL)
		if err != nil {
			return nil, err
		}
		if existing, found := seenEnvs[env.ID]; found {
			return nil, errors.E(env.Info, "environment '%s' already defined at %s", env.ID, existing.Info.String())
		}
		if env.PromoteFrom != "" {
			if _, found := seenEnvs[env.PromoteFrom]; !found {
				return nil, errors.E(envHCL.PromoteFrom.Range, "environment.promote_from '%s' does not exist", env.PromoteFrom)
			}
		}
		result = append(result, env)
		seenEnvs[env.ID] = env
	}

	return result, nil
}

func evalEnvironment(evalctx *eval.Context, envHCL *hcl.Environment) (*Environment, error) {
	env := &Environment{
		Info: envHCL.Info,
	}
	var err error

	// Required: ID
	if envHCL.ID == nil {
		return nil, errors.E(envHCL.Info, "environment block is missing required attribute 'id'")
	}
	env.ID, err = EvalString(evalctx, envHCL.ID.Expr, "id")
	if err != nil {
		return nil, err
	}

	// Required: Name
	if envHCL.Name == nil {
		return nil, errors.E(envHCL.Info, "environment block is missing required attribute 'name'")
	}
	env.Name, err = EvalString(evalctx, envHCL.Name.Expr, "name")
	if err != nil {
		return nil, err
	}

	// Optional: Description
	if envHCL.Description != nil {
		env.Description, err = EvalString(evalctx, envHCL.Description.Expr, "description")
		if err != nil {
			return nil, err
		}
	}

	// Optional: PromoteFrom
	if envHCL.PromoteFrom != nil {
		env.PromoteFrom, err = EvalString(evalctx, envHCL.PromoteFrom.Expr, "promote_from")
		if err != nil {
			return nil, err
		}
	}

	return env, nil
}
