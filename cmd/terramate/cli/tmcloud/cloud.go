// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tmcloud

import (
	"os"

	"github.com/terramate-io/terramate/cloud"
)

// BaseURL returns the base URL for the TMC API.
func BaseURL() string {
	var baseURL string
	cloudHost := os.Getenv("TMC_API_HOST")
	cloudURL := os.Getenv("TMC_API_URL")
	if cloudHost != "" {
		baseURL = "https://" + cloudHost
	} else if cloudURL != "" {
		baseURL = cloudURL
	} else {
		baseURL = cloud.BaseURL
	}
	return baseURL
}
