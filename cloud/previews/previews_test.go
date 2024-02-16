// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package previews_test

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/previews"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/project"
)

func TestCreatePreview(t *testing.T) {
	type want struct {
		numStacksReturned   int
		httpEndpointsCalled []string
		err                 error
	}
	type testcase struct {
		name           string
		numRunContexts int
		orgUUID        string
		want           want
	}

	testcases := []testcase{
		{
			name:           "zero run contexts",
			orgUUID:        "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			numRunContexts: 0,
			want: want{
				numStacksReturned:   0,
				httpEndpointsCalled: []string{},
				err:                 errors.E("no affected stacks or runs provided"),
			},
		},
		{
			name:           "empty org uuid returns an error",
			orgUUID:        "",
			numRunContexts: 3,
			want: want{
				numStacksReturned:   0,
				httpEndpointsCalled: []string{},
				err:                 errors.E("org uuid is empty"),
			},
		},
		{
			name:           "three stacks",
			orgUUID:        "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			numRunContexts: 3,
			want: want{
				numStacksReturned: 3,
				httpEndpointsCalled: []string{
					"POST /v1/previews/aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
				},
				err: nil,
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			testTransport := &previewsTransport{}
			client := &cloud.Client{
				Credential: credential{"sometoken"},
				HTTPClient: &http.Client{Transport: testTransport},
			}

			now := time.Now().UTC()
			opts := previews.CreatePreviewOpts{
				Runs: makeRunContexts(tc.numRunContexts,
					[]string{"terraform", "plan", "-out", "plan.tfout"}, true),
				AffectedStacks:  makeAffectedStacks(tc.numRunContexts, true),
				OrgUUID:         cloud.UUID(tc.orgUUID),
				UpdatedAt:       now.Unix(),
				Technology:      "terraform",
				TechnologyLayer: "default",
				Repository:      "https://github.com/owner/repo",
				DefaultBranch:   "main",
				ReviewRequest: &cloud.DeploymentReviewRequest{
					Platform:    "github",
					Repository:  "https://github.com/owner/repo",
					CommitSHA:   "2fef3ab48c543322e911bc53baec6196231e95bc",
					Number:      23,
					Title:       "feat: add stacks",
					Description: "Added some stacks",
					URL:         "https://github.com/owner/repo/pull/23",
					Labels:      []cloud.Label{},
					Reviewers:   []cloud.Reviewer{},
					PushedAt:    &now,
				},
				Metadata: &cloud.DeploymentMetadata{},
			}

			createdPreview, err := previews.CreatePreview(client, time.Second*2, opts)
			if err != tc.want.err {
				assert.EqualStrings(t, tc.want.err.Error(), err.Error(),
					"unexpected error")
				return
			}

			assert.EqualInts(t, len(createdPreview.StackPreviewsByMetaID), tc.want.numStacksReturned,
				"unexpected number of stacks returned")
			assert.EqualInts(t, len(testTransport.receivedReqs), len(tc.want.httpEndpointsCalled),
				"unexpected number of HTTP requests")

			if diff := cmp.Diff(testTransport.httpEndpointsCalled(), tc.want.httpEndpointsCalled); diff != "" {
				t.Errorf("unexpected HTTP reqs: %s", diff)
			}
		})
	}
}

func makeAffectedStacks(num int, isChanged bool) map[string]*config.Stack {
	affectedStacks := map[string]*config.Stack{}
	for i := 0; i < num; i++ {
		affectedStacks[strconv.Itoa(i)] = &config.Stack{
			Dir:         project.NewPath("/somepath" + strconv.Itoa(i)),
			ID:          uuid.NewString(),
			Name:        "stack" + strconv.Itoa(i),
			Description: "desc of stack" + strconv.Itoa(i),
			Tags:        []string{"tag1", "tag2"},
			IsChanged:   isChanged,
		}
	}

	return affectedStacks
}

func makeRunContexts(num int, cmd []string, isChanged bool) []previews.RunContext {
	runs := make([]previews.RunContext, num)
	for i := 0; i < num; i++ {
		runs[i] = previews.RunContext{
			Stack: &config.Stack{
				Dir:         project.NewPath("/somepath" + strconv.Itoa(i)),
				ID:          uuid.NewString(),
				Name:        "stack" + strconv.Itoa(i),
				Description: "desc of stack" + strconv.Itoa(i),
				Tags:        []string{"tag1", "tag2"},
				IsChanged:   isChanged,
			},
			Cmd: cmd,
		}
	}

	return runs
}

type credential struct {
	token string
}

func (c credential) Token() (string, error) {
	return c.token, nil
}
