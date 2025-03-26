// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package testserver

import (
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/julienschmidt/httprouter"
	"github.com/terramate-io/terramate/cloud/api/resources"
	"github.com/terramate-io/terramate/cloud/testserver/cloudstore"
	"github.com/terramate-io/terramate/errors"
)

// GetUsers implements the /v1/users endpoint.
func GetUsers(store *cloudstore.Data, w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	user, found, err := userFromRequest(store, r)
	if err != nil || !found {
		w.WriteHeader(http.StatusUnauthorized)
		writeErr(w, err)
		return
	}
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	marshalWrite(w, user)
}

func userFromRequest(store *cloudstore.Data, r *http.Request) (resources.User, bool, error) {
	authorization := r.Header.Get("Authorization")
	if authorization == "" {
		return resources.User{}, false, nil
	}
	if !strings.HasPrefix(authorization, "Bearer ") {
		return resources.User{}, false, nil
	}
	tokenStr := strings.TrimPrefix(authorization, "Bearer ")
	if tokenStr == "" {
		return resources.User{}, true, errors.E("no bearer token")
	}

	var jwtParser jwt.Parser

	claims := jwt.MapClaims{}
	_, _, err := jwtParser.ParseUnverified(tokenStr, claims)
	if err != nil {
		return resources.User{}, true, errors.E(err, "parsing jwt token")
	}

	email := claims["email"].(string)
	user, found := store.GetUser(email)
	if !found {
		return resources.User{}, true, errors.E("email %s not found", email)
	}
	return user, true, nil
}
