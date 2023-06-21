// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build !defaultAudience

package cli

import "github.com/terramate-io/terramate/cloud"

func oidcAudience() string {
	return cloud.Host
}
