// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud

import (
	"context"
	stdhttp "net/http"
	"path"

	"testing"
	"time"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/api/resources"
	"github.com/terramate-io/terramate/http"
	"github.com/terramate-io/terramate/strconv"
)

const defaultTestTimeout = 1 * time.Second

// PutStack sets a new stack in the /v1/stacks/<org>/<stack id>.
// Note: this is not a real endpoint.
func PutStack(t *testing.T, addr string, orgUUID resources.UUID, st resources.StackObject) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), defaultTestTimeout)
	defer cancel()

	client := cloud.NewClient(
		cloud.WithBaseURL("http://"+addr),
		cloud.WithCredential(&credential{}),
	)

	_, err := http.Put[resources.EmptyResponse](ctx, client, st, client.URL(path.Join(cloud.StacksPath, string(orgUUID), strconv.Itoa64(st.ID))))
	assert.NoError(t, err)
}

type credential struct{}

func (c *credential) ApplyCredentials(req *stdhttp.Request) error {
	req.Header.Set("Authorization", "Bearer abcd")
	return nil
}

func (c *credential) RedactCredentials(_ *stdhttp.Request) {}
