// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloudstore_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/testserver/cloudstore"
)

const testUUID = "deadbeef-dead-dead-dead-deaddeafbeef"

func TestGetUser(t *testing.T) {
	expectedUser := cloud.User{
		UUID:        testUUID,
		Email:       "batman@terramate.io",
		DisplayName: "Batman",
		JobTitle:    "Entrepreneur",
	}
	dstore := cloudstore.Data{
		Users: map[string]cloud.User{
			"other": {
				UUID:        "88ae6cb4-ee56-40aa-a024-84af44e1f5aa",
				Email:       "other@other.io",
				DisplayName: "other",
				JobTitle:    "other",
			},
			"batman": expectedUser,
		},
	}
	_, found := dstore.GetUser("nonexistent@terramate.io")
	if found {
		t.Fatal("user must not exist")
	}
	user, found := dstore.GetUser("batman@terramate.io")
	if !found {
		t.Fatal("user must exist")
	}
	if diff := cmp.Diff(expectedUser, user); diff != "" {
		t.Fatal(diff)
	}
}
