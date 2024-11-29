// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build !localhostEndpoints

package telemetry

import (
	"net/url"
)

func Endpoint() url.URL {
	var u url.URL
	u.Scheme = "https"
	u.Host = "analytics.terramate.io"
	u.Path = "/"
	return u
}
