// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package commands

type EngineRequirement struct {
	LoadTerragrunt bool
}

func RequireEngine(opts ...RequireEngineOpt) *EngineRequirement {
	req := &EngineRequirement{}
	for _, opt := range opts {
		opt(req)
	}
	return req
}

type RequireEngineOpt func(*EngineRequirement)

func WithLoadTerragrunt() RequireEngineOpt {
	return func(req *EngineRequirement) {
		req.LoadTerragrunt = true
	}
}

type ExperimentsRequirement struct {
	Names []string
}

func RequireExperiments(names ...string) *ExperimentsRequirement {
	return &ExperimentsRequirement{Names: names}
}
