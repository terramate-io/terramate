// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/terramate-io/terramate/cloud"
)

func (orgHandler *organizationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/v1/organizations" {
		panic("unsupported")
	}
	w.Header().Add("Content-Type", "application/json")
	if r.Method == "GET" {
		_, _ = w.Write([]byte(
			`[
		{
			"org_name": "terramate-io",
			"org_display_name": "Terramate",
			"org_uuid": "0000-1111-2222-3333"
		}
	]`,
		))
		return
	}

	panic("unsupported")
}

func (userHandler *userHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/v1/users" {
		panic("unsupported")
	}
	w.Header().Add("Content-Type", "application/json")
	if r.Method == "GET" {
		_, _ = w.Write([]byte(
			`{
			    "email": "batman@example.com",
			    "display_name": "batman",
				"job_title": "entrepreneur"
			}`,
		))
		return
	}

	panic("unsupported")
}

func (deploymentHandler *deploymentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("request: %+v\n", r.URL)
	w.Header().Add("Content-Type", "application/json")
	if r.Method == "POST" {
		defer func() { _ = r.Body.Close() }()
		data, _ := io.ReadAll(r.Body)
		fmt.Printf("request payload: %s\n", data)
		var p cloud.DeploymentStacksPayloadRequest
		err := json.Unmarshal(data, &p)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(err.Error()))
			return
		}

		res := cloud.DeploymentStacksResponse{}
		for _, s := range p.Stacks {
			res = append(res, cloud.DeploymentStackResponse{
				StackID: s.MetaID,
				Status:  cloud.Pending,
			})
		}
		data, _ = json.Marshal(res)
		_, _ = w.Write(data)
		return
	}

	panic("unsupported")
}

type (
	userHandler         struct{}
	organizationHandler struct{}
	deploymentHandler   struct{}
)
