// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud

import (
	"context"
	"path"

	"testing"
	"time"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/strconv"
)

const defaultTestTimeout = 1 * time.Second

// PutStack sets a new stack in the /v1/stacks/<org>/<stack id>.
// Note: this is not a real endpoint.
func PutStack(t *testing.T, addr string, orgUUID cloud.UUID, st cloud.StackObject) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), defaultTestTimeout)
	defer cancel()

	client := &cloud.Client{
		BaseURL:    "http://" + addr,
		Credential: &credential{},
	}
	_, err := cloud.Put[cloud.EmptyResponse](ctx, client, st, client.URL(path.Join(cloud.StacksPath, string(orgUUID), strconv.Itoa64(st.ID))))
	assert.NoError(t, err)
}

type credential struct{}

func (c *credential) Token() (string, error) { return "abcd", nil }
