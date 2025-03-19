// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build !localhostEndpoints

package tui

import "net/url"

func defaultTelemetryEndpoint() url.URL {
	var u url.URL
	u.Scheme = "https"
	u.Host = "checkpoint-api.mineiros.io"
	u.Path = "/"
	return u
}
