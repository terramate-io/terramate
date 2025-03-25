// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cliauth

import "os"

// EnvBaseURL returns the base URL for the TMC API.
func EnvBaseURL() (string, bool) {
	if cloudHost := os.Getenv("TMC_API_HOST"); cloudHost != "" {
		return "https://" + cloudHost, true
	}
	if cloudURL := os.Getenv("TMC_API_URL"); cloudURL != "" {
		return cloudURL, true
	}
	return "", false
}
