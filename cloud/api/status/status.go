// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package status provides utilities for parsing Terramate Cloud status filters.
package status

import (
	"github.com/terramate-io/terramate/cloud/api/deployment"
	"github.com/terramate-io/terramate/cloud/api/drift"
	"github.com/terramate-io/terramate/cloud/api/resources"
	"github.com/terramate-io/terramate/cloud/api/stack"
	"github.com/terramate-io/terramate/errors"
)

// ParseFilters parses the set of Terramate Cloud filters and return an error if any of them
// is not recognized. If any argument is an empty string then it returns its corresponding <type>.NoFilter.
func ParseFilters(stackStatus, deploymentStatus, driftStatus string) (resources.StatusFilters, error) {
	stackStatusFilter, err := parseStackStatusFilter(stackStatus)
	if err != nil {
		return resources.NoStatusFilters(), err
	}
	deploymentStatusFilter, err := parseDeploymentStatusFilter(deploymentStatus)
	if err != nil {
		return resources.NoStatusFilters(), err
	}
	driftStatusFilter, err := parseDriftStatusFilter(driftStatus)
	if err != nil {
		return resources.NoStatusFilters(), err
	}
	return resources.StatusFilters{
		StackStatus:      stackStatusFilter,
		DeploymentStatus: deploymentStatusFilter,
		DriftStatus:      driftStatusFilter,
	}, nil
}

func parseStackStatusFilter(filterStr string) (stack.FilterStatus, error) {
	if filterStr == "" {
		return stack.NoFilter, nil
	}
	filter, err := stack.NewStatusFilter(filterStr)
	if err != nil {
		return stack.NoFilter, errors.E(err, "unrecognized stack filter")
	}
	return filter, nil
}

func parseDeploymentStatusFilter(filterStr string) (deployment.FilterStatus, error) {
	if filterStr == "" {
		return deployment.NoFilter, nil
	}
	filter, err := deployment.NewStatusFilter(filterStr)
	if err != nil {
		return deployment.NoFilter, errors.E(err, "unrecognized deployment filter")
	}
	return filter, nil
}

func parseDriftStatusFilter(filterStr string) (drift.FilterStatus, error) {
	if filterStr == "" {
		return drift.NoFilter, nil
	}
	filter, err := drift.NewStatusFilter(filterStr)
	if err != nil {
		return drift.NoFilter, errors.E(err, "unrecognized drift filter")
	}
	return filter, nil
}
