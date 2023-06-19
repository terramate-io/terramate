// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package main implements the cloudmock service.
package main

import "net/http"

func main() {
	mux := http.NewServeMux()
	mux.Handle("/v1/users", &userHandler{})
	mux.Handle("/v1/organizations", &organizationHandler{})
	mux.Handle("/v1/deployments/0000-1111-2222-3333/0000-1111-2222-3333/stacks", &deploymentHandler{})
	s := &http.Server{
		Addr:    "localhost:3001",
		Handler: mux,
	}

	err := s.ListenAndServe()
	if err != nil {
		panic(err)
	}
}
