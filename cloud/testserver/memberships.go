// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package testserver

import (
	"fmt"
	"net/http"
)

type membershipHandler struct{}

func newMembershipEndpoint() *membershipHandler {
	return &membershipHandler{}
}

func (orgHandler *membershipHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	writeString(w, fmt.Sprintf(`[
		{
			"org_name": "terramate-io",
			"org_display_name": "Terramate",
			"org_uuid": "%s",
			"status": "active"
		}
	]`, DefaultOrgUUID),
	)
}
