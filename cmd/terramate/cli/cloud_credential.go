// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

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
