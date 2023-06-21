// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package main implements the cloudmock service.
package main

import (
	"net/http"

	"github.com/terramate-io/terramate/cloud/testserver"
)

func main() {
	s := &http.Server{
		Addr:    "localhost:3001",
		Handler: testserver.Router(),
	}

	err := s.ListenAndServe()
	if err != nil {
		panic(err)
	}
}
