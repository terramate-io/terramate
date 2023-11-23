// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package main implements the cloudmock service.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/terramate-io/terramate/cloud/testserver"
	"github.com/terramate-io/terramate/cloud/testserver/cloudstore"
)

func main() {
	datafile := "testdata/testserver/cloud.data.json"
	if len(os.Args) == 2 {
		datafile = os.Args[1]
	}
	store, err := cloudstore.LoadDatastore(datafile)
	if err != nil {
		panic(err)
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Fprintf(os.Stderr, "%s\n", data)
	s := &http.Server{
		Addr:    "0.0.0.0:3001",
		Handler: testserver.Router(store),
	}

	fmt.Printf("listening at %s\n", s.Addr)
	err = s.ListenAndServe()
	if err != nil {
		panic(err)
	}
}
