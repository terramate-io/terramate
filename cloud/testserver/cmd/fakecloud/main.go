// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package main implements the cloudmock service.
package main

import (
	"fmt"
	"net/http"

	"github.com/terramate-io/terramate/cloud/testserver"
)

func main() {
	s := &http.Server{
		Addr:    "0.0.0.0:3001",
		Handler: testserver.Router(),
	}

	fmt.Printf("listening at %s\n", s.Addr)
	err := s.ListenAndServe()
	if err != nil {
		panic(err)
	}
}
