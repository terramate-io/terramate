// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud

import (
	"bytes"

	"github.com/terramate-io/terramate/errors"
)

type (
	// MemberOrganizations is a list of organizations associated with the member.
	MemberOrganizations []MemberOrganization

	// MemberOrganization represents the organization associated with the member.
	MemberOrganization struct {
		MemberID    int    `json:"member_id,omitempty"`
		Name        string `json:"org_name"`
		DisplayName string `json:"org_display_name"`
		Domain      string `json:"org_domain"`
		UUID        string `json:"org_uuid"`
		Role        string `json:"role,omitempty"`
		Status      string `json:"status"`
	}
)

// String representation of the list of organization associated with the user.
func (orgs MemberOrganizations) String() string {
	var out bytes.Buffer

	write := func(s string) {
		// only possible error is OutOfMemory which panics already
		_, _ = out.Write([]byte(s))
	}

	if len(orgs) == 0 {
		write("none")
	} else {
		for i, org := range orgs {
			write(org.DisplayName)
			if i+1 < len(orgs) {
				write(", ")
			}
		}
	}
	return out.String()
}

// Validate if the organization list is valid.
func (orgs MemberOrganizations) Validate() error {
	for _, org := range orgs {
		err := org.Validate()
		if err != nil {
			return err
		}
	}
	return nil
}

// Validate checks if at least the fields required by Terramate CLI are set.
func (org MemberOrganization) Validate() error {
	if org.Name == "" {
		return errors.E(`missing "name" field`)
	}
	if org.UUID == "" {
		return errors.E(`missing "org_uuid" field`)
	}
	return nil
}
