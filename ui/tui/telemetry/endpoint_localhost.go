// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build localhostEndpoints

package telemetry

import (
	"net/url"
)

func Endpoint() url.URL {
	var u url.URL
	u.Scheme = "http"
	u.Host = "localhost:3000"
	u.Path = "/"
	return u
}
