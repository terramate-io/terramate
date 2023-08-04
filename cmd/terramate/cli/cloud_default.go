// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build !localhostEndpoints

package cli

import (
	"github.com/hashicorp/go-uuid"
	"github.com/terramate-io/terramate/cloud"
)

const cloudDefaultBaseURL = cloud.BaseURL

func generateRunID() (string, error) {
	return uuid.GenerateUUID()
}
