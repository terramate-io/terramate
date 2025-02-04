// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package auth

import "net/http"

type providerID string

func (p providerID) String() string {
	switch p {
	case "google.com":
		return "Google"
	case "github.com":
		return "GitHub"
	default:
		return string(p)
	}
}

func applyJWTBasedCredentials(req *http.Request, cred Credential) error {
	token, err := cred.Token()
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return nil
}

func redactJWTBasedCredentials(req *http.Request) {
	req.Header.Set("Authorization", "Bearer REDACTED")
}
