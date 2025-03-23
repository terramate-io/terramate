// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package testserver

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/terramate-io/terramate/cloud/api/resources"
	"github.com/terramate-io/terramate/cloud/testserver/cloudstore"
	"github.com/terramate-io/terramate/errors"
)

// GetMemberships is the testserver GET /memberships handler.
func GetMemberships(store *cloudstore.Data, w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	user, found, err := userFromRequest(store, r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		writeErr(w, err)
		return
	}
	var memberships []cloudstore.Member
	if found {
		memberships = store.GetMemberships(user)
	} else {
		key, found, err := apikeyFromRequest(r)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			writeErr(w, err)
			return
		}

		if !found {
			w.WriteHeader(http.StatusUnauthorized)
			writeErr(w, errors.E("no valid authentication method"))
			return
		}

		memberships = store.GetMembershipsForKey(key)
	}
	var retMemberships resources.MemberOrganizations
	for _, member := range memberships {
		retMemberships = append(retMemberships, resources.MemberOrganization{
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
