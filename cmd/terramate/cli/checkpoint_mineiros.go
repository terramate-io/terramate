// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build !localhostTelemetry

package cli

import "net/url"

func defaultTelemetryEndpoint() url.URL {
	var u url.URL
	u.Scheme = "https"
	u.Host = "checkpoint-api.mineiros.io"
	u.Path = "/"
	return u
}
