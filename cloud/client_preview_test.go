// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	stdhttp "net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/api/preview"
	"github.com/terramate-io/terramate/cloud/api/resources"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/project"
)

func TestCreatePreview(t *testing.T) {
	t.Parallel()
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
				numStacksReturned: 0,
				httpEndpointsCalled: []string{
					"POST /v1/previews/aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
				},
				err: nil,
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
			testTransport := &previewTransport{}
			client := cloud.NewClient(
				cloud.WithHTTPClient(&stdhttp.Client{Transport: testTransport}),
				cloud.WithCredential(credential()),
			)

			now := time.Now().UTC()
			opts := cloud.CreatePreviewOpts{
				Runs:            makeRunContexts(tc.numRunContexts, []string{"terraform", "plan", "-out", "plan.tfout"}),
				AffectedStacks:  makeAffectedStacks(tc.numRunContexts),
				OrgUUID:         resources.UUID(tc.orgUUID),
				PushedAt:        now.Unix(),
				CommitSHA:       "2fef3ab48c543322e911bc53baec6196231e95bc",
				Technology:      "terraform",
				TechnologyLayer: "default",
				ReviewRequest: &resources.ReviewRequest{
					Platform:    "github",
					Repository:  "https://github.com/owner/repo",
					CommitSHA:   "2fef3ab48c543322e911bc53baec6196231e95bc",
					Number:      23,
					Title:       "feat: add stacks",
					Description: "Added some stacks",
					URL:         "https://github.com/owner/repo/pull/23",
					Labels:      []resources.Label{},
					Reviewers:   []resources.Reviewer{},
					UpdatedAt:   &now,
				},
				Metadata: &resources.DeploymentMetadata{},
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
			defer cancel()
			createdPreview, err := client.CreatePreview(ctx, opts)
			if err != tc.want.err {
				assert.EqualErrs(t, tc.want.err, err,
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

func TestUpdateStackPreview(t *testing.T) {
	t.Parallel()
	type want struct {
		httpEndpointsCalled []string
		err                 error
	}
	type testcase struct {
		name           string
		orgUUID        string
		stackPreviewID string
		status         preview.StackStatus
		want           want
	}

	testcases := []testcase{
		{
			name:           "stack preview status running",
			orgUUID:        "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			stackPreviewID: "123",
			status:         "running",
			want: want{
				httpEndpointsCalled: []string{
					"PATCH /v1/stack_previews/aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee/123",
				},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			testTransport := &previewTransport{}
			client := cloud.NewClient(
				cloud.WithHTTPClient(&stdhttp.Client{Transport: testTransport}),
				cloud.WithCredential(credential()),
			)

			opts := cloud.UpdateStackPreviewOpts{
				OrgUUID:        resources.UUID(tc.orgUUID),
				StackPreviewID: tc.stackPreviewID,
				Status:         tc.status,
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
			defer cancel()
			err := client.UpdateStackPreview(ctx, opts)
			assert.EqualErrs(t, tc.want.err, err, "unexpected error")
			assert.EqualInts(t, len(testTransport.receivedReqs), len(tc.want.httpEndpointsCalled),
				"unexpected number of HTTP requests")

			if diff := cmp.Diff(testTransport.httpEndpointsCalled(), tc.want.httpEndpointsCalled); diff != "" {
				t.Errorf("unexpected HTTP reqs: %s", diff)
			}
		})
	}
}

func makeAffectedStacks(num int) map[string]resources.Stack {
	affectedStacks := map[string]resources.Stack{}
	for i := 0; i < num; i++ {
		affectedStacks[strconv.Itoa(i)] = resources.Stack{
			Path:            project.NewPath("/somepath" + strconv.Itoa(i)).String(),
			MetaID:          uuid.NewString(),
			MetaName:        "stack" + strconv.Itoa(i),
			MetaDescription: "desc of stack" + strconv.Itoa(i),
			MetaTags:        []string{"tag1", "tag2"},
			Repository:      "https://github.com/owner/repo",
			DefaultBranch:   "main",
		}
	}

	return affectedStacks
}

func makeRunContexts(num int, cmd []string) []cloud.RunContext {
	runs := make([]cloud.RunContext, num)
	for i := 0; i < num; i++ {
		runs[i] = cloud.RunContext{
			StackID: uuid.NewString(),
			Cmd:     cmd,
		}
	}

	return runs
}

type previewTransport struct {
	// receivedReqs is a list of all requests that were processed by the transport.
	receivedReqs []*stdhttp.Request
}

func (f *previewTransport) RoundTrip(req *stdhttp.Request) (*stdhttp.Response, error) {
	f.receivedReqs = append(f.receivedReqs, req)

	endpoint := req.Method + " " + req.URL.Path
	switch {
	case strings.HasPrefix(endpoint, "POST /v1/previews"):
		return createPreviewsHandler(req)
	case strings.HasPrefix(endpoint, "PATCH /v1/stack_previews"):
		return updateStackPreviewsHandler(req)
	default:
		return &stdhttp.Response{
			StatusCode: stdhttp.StatusNotFound,
		}, nil
	}

}

// updateStackPreviewsHandler is a handler for the PATCH /v1/stack_previews endpoint.
func updateStackPreviewsHandler(req *stdhttp.Request) (*stdhttp.Response, error) {
	var reqParsed resources.UpdateStackPreviewPayloadRequest
	if err := json.NewDecoder(req.Body).Decode(&reqParsed); err != nil {
		return &stdhttp.Response{StatusCode: stdhttp.StatusInternalServerError}, nil
	}

	validStatuses := []string{"running", "changed", "unchanged", "failed", "canceled"}
	if !slices.Contains(validStatuses, reqParsed.Status) {
		return &stdhttp.Response{
			StatusCode: stdhttp.StatusBadRequest,
		}, nil
	}

	return &stdhttp.Response{
		StatusCode: stdhttp.StatusNoContent,
	}, nil
}

// createPreviewsHandler is a handler for the POST /v1/previews endpoint.
func createPreviewsHandler(req *stdhttp.Request) (*stdhttp.Response, error) {
	var reqParsed resources.CreatePreviewPayloadRequest
	if err := json.NewDecoder(req.Body).Decode(&reqParsed); err != nil {
		return &stdhttp.Response{StatusCode: stdhttp.StatusInternalServerError}, nil
	}

	resp := resources.CreatePreviewResponse{PreviewID: "1", Stacks: []resources.ResponsePreviewStack{}}
	for i, s := range reqParsed.Stacks {
		resp.Stacks = append(resp.Stacks, resources.ResponsePreviewStack{
			MetaID:         s.MetaID,
			StackPreviewID: strconv.Itoa(i),
		})
	}

	respBytes, err := json.Marshal(resp)
	if err != nil {
		return &stdhttp.Response{StatusCode: stdhttp.StatusInternalServerError}, nil
	}

	return &stdhttp.Response{
		StatusCode: stdhttp.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(respBytes)),
		Header:     stdhttp.Header{"Content-Type": []string{"application/json"}},
	}, nil
}

func (f *previewTransport) httpEndpointsCalled() []string {
	endpoints := make([]string, len(f.receivedReqs))
	for i, r := range f.receivedReqs {
		endpoints[i] = fmt.Sprintf("%s %s", r.Method, r.URL.Path)
	}
	return endpoints
}
