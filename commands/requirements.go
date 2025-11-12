// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package commands

// EngineRequirement is a requirement for commands that need the configuration to be loaded.
// Commands that don't have this requirement set may not call CLI.Engine().
type EngineRequirement struct {
	LoadTerragruntModules bool
	Experiments           []string
}

// RequireEngine creates a new EngineRequirement with the given options.
func RequireEngine(opts ...RequireEngineOpt) *EngineRequirement {
	req := &EngineRequirement{}
	for _, opt := range opts {
		opt(req)
	}
	return req
}

// RequireEngineOpt is the option type for RequireEngine().
type RequireEngineOpt func(*EngineRequirement)

// WithTerragrunt an option for RequireEngine() to control loading of Terragrunt modules
// for change detection and dependency tracking.
func WithTerragrunt(loadModules bool) RequireEngineOpt {
	return func(req *EngineRequirement) {
		req.LoadTerragruntModules = loadModules
	}
}

// WithExperiments an option for RequireEngine() to require given experiments to be enabled.
// This is part of the engine requirement, because experiments are specified in the config.
func WithExperiments(names ...string) RequireEngineOpt {
	return func(req *EngineRequirement) {
		req.Experiments = names
	}
}
