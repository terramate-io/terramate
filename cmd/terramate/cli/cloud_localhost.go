// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build localhostEndpoints

package cli

import (
	"github.com/hashicorp/go-uuid"
	cloudtest "github.com/terramate-io/terramate/test/cloud"
	"os"
)

const cloudDefaultBaseURL = cloudtest.TestEndpoint

func generateRunID() (string, error) {
	if runid := os.Getenv("TM_TEST_RUN_ID"); runid != "" {
		return runid, nil
	}
	return uuid.GenerateUUID()
}
