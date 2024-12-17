// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package testserver

import (
	"net/http"
	"strings"

	"github.com/terramate-io/terramate/errors"
)

func apikeyFromRequest(r *http.Request) (key string, found bool, err error) {
	authorization := r.Header.Get("Authorization")
	if authorization == "" {
		return "", false, nil
	}

	if !strings.HasPrefix(authorization, "Basic ") {
		return "", false, nil
	}

	key, notExpectedPass, ok := r.BasicAuth()
	if !ok {
		return "", true, errors.E("invalid basic auth")
	}
	if notExpectedPass != "" {
		return "", true, errors.E("API key must not have a password set")
	}
	return key, true, nil
}
