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
