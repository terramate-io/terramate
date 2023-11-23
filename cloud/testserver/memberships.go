// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package testserver

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/testserver/cloudstore"
)

// GetMemberships is the testserver GET /memberships handler.
func GetMemberships(store *cloudstore.Data, w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	user, err := userFromRequest(store, r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		writeErr(w, err)
		return
	}
	memberships := store.GetMemberships(user)
	var retMemberships cloud.MemberOrganizations
	for _, member := range memberships {
		retMemberships = append(retMemberships, cloud.MemberOrganization{
			MemberID:    member.MemberID,
			Name:        member.Org.Name,
			DisplayName: member.Org.DisplayName,
			Domain:      member.Org.Domain,
			UUID:        member.Org.UUID,
			Role:        member.Role,
			Status:      member.Status,
		})
	}
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	marshalWrite(w, retMemberships)
}
