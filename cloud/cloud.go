// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package cloud implements a client SDK for communication with the cloud API.
package cloud

import (
	"os"
	"strings"
	"time"

	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/strconv"
)

// BaseDomain is the Terramate Cloud base domain.
const apiBaseDomain = "api.terramate.io"

const defaultPageSize = 50

const (
	// WellKnownCLIPath is the well-known base path.
	WellKnownCLIPath = "/.well-known/cli.json"

	// SingleSignOnDetailByNamePath is the path to the organization SSO details.
	SingleSignOnDetailByNamePath = "/v1/organizations/name"

	// UsersPath is the users endpoint base path.
	UsersPath = "/v1/users"
	// MembershipsPath is the memberships endpoint base path.
	MembershipsPath = "/v1/memberships"
	// DeploymentsPath is the deployments endpoint base path.
	DeploymentsPath = "/v1/deployments"
	// DriftsV1Path is the drifts V1 endpoint base path.
	DriftsV1Path = "/v1/drifts"
	// DriftsPath is the drifts endpoint base path.
	DriftsPath = "/v2/drifts"
	// StacksPath is the stacks endpoint base path.
	StacksPath = "/v1/stacks"
	// ReviewRequestsPath is the review requests endpoint base path.
	ReviewRequestsPath = "/v1/review_requests"
	// StorePath is the store endpoint base path.
	StorePath = "/v1/store"
)

// DefaultTimeout is a (optional) good default timeout to be used by TMC clients.
const DefaultTimeout = 60 * time.Second

type (
	// Region is the Terramate Cloud region (EU, US, etc).
	Region int

	// Regions is a list of cloud regions.
	Regions []Region
)

// Available cloud locations.
const (
	// For backward compatibility we want the zero value to be the default
	// if not set in the [cloud.Client] struct.
	EU Region = iota
	US
	invalidRegion
)

var (
	pageSize int64 = defaultPageSize
)

func init() {
	if sizeStr := os.Getenv("TMC_API_PAGESIZE"); sizeStr != "" {
		size, _ := strconv.Atoi64(sizeStr)
		if size != 0 {
			pageSize = int64(size)
		}
	}
}

// BaseURL returns the official API base URL for the Terramate Cloud.
func BaseURL(region Region) string {
	if region == EU {
		return "https://" + apiBaseDomain
	}
	return "https://" + region.String() + "." + apiBaseDomain
}

// BaseDomain returns the official API base domain for the Terramate Cloud.
func BaseDomain(region Region) string {
	if region == EU {
		return apiBaseDomain
	}
	return region.String() + "." + apiBaseDomain
}

// ParseRegion parses a user-supplied region name.
func ParseRegion(str string) (Region, error) {
	switch str {
	case "eu":
		return EU, nil
	case "us":
		return US, nil
	default:
		return invalidRegion, errors.E("unknown cloud region: %s", str)
	}
}

// String returns the string representation of the region.
func (r Region) String() string {
	switch r {
	case EU:
		return "eu"
	case US:
		return "us"
	default:
		panic(errors.E("invalid region", r))
	}
}

// String returns the string representation of the regions list.
func (rs Regions) String() string {
	var regions []string
	for _, r := range rs {
		regions = append(regions, r.String())
	}
	return strings.Join(regions, ", ")
}

// AvailableRegions returns a list of available cloud regions.
func AvailableRegions() Regions {
	return Regions{EU, US}
}

// HTMLURL returns the Terramate Cloud frontend URL.
func HTMLURL(region Region) string {
	if region == EU {
		return "https://cloud.terramate.io"
	}
	return "https://" + region.String() + ".cloud.terramate.io"
}

// Entity defines the cloud possible entities with its identifier.
type Entity struct {
	Kind     EntityKind
	EntityID string
}

// EntityKind are the types of Entities that exist.
type EntityKind int

const (
	// EntityKindDeployment is the deployment sync entity.
	EntityKindDeployment EntityKind = iota + 1
	// EntityKindDrift is the drift check sync entity.
	EntityKindDrift
	// EntityKindPreview is the preview sync entity.
	EntityKindPreview
)

// String returns the string representing an EntityKind.
func (k EntityKind) String() string {
	switch k {
	case EntityKindDeployment:
		return "deployment"
	case EntityKindPreview:
		return "preview"
	case EntityKindDrift:
		return "drift"
	}
	return "unknown"
}
