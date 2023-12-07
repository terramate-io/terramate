// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package testserver

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/terramate-io/terramate/cloud/testserver/cloudstore"
)

// GetWellKnown implements the /.well-known/cli.json endpoint.
func GetWellKnown(store *cloudstore.Data, w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	wk := store.GetWellKnown()
	if wk == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	marshalWrite(w, wk)
}
