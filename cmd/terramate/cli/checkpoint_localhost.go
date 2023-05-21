// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build localhostTelemetry

package cli

import "net/url"

func defaultTelemetryEndpoint() url.URL {
	var u url.URL
	u.Scheme = "http"
	u.Host = "localhost:3000"
	u.Path = "/"
	return u
}
