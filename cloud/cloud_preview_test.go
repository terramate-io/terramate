// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/preview"
	"github.com/terramate-io/terramate/config"
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
			client := &cloud.Client{
				Credential: credential(),
				HTTPClient: &http.Client{Transport: testTransport},
			}

			now := time.Now().UTC()
			opts := cloud.CreatePreviewOpts{
				Runs:            makeRunContexts(tc.numRunContexts, []string{"terraform", "plan", "-out", "plan.tfout"}),
				AffectedStacks:  makeAffectedStacks(tc.numRunContexts),
				OrgUUID:         cloud.UUID(tc.orgUUID),
				UpdatedAt:       now.Unix(),
				PushedAt:        now.Unix(),
				CommitSHA:       "2fef3ab48c543322e911bc53baec6196231e95bc",
				Technology:      "terraform",
				TechnologyLayer: "default",
				Repository:      "https://github.com/owner/repo",
				DefaultBranch:   "main",
				ReviewRequest: &cloud.ReviewRequest{
					Platform:    "github",
					Repository:  "https://github.com/owner/repo",
					CommitSHA:   "2fef3ab48c543322e911bc53baec6196231e95bc",
					Number:      23,
					Title:       "feat: add stacks",
					Description: "Added some stacks",
					URL:         "https://github.com/owner/repo/pull/23",
					Labels:      []cloud.Label{},
					Reviewers:   []cloud.Reviewer{},
					UpdatedAt:   &now,
				},
				Metadata: &cloud.DeploymentMetadata{},
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
			client := &cloud.Client{
				Credential: credential(),
				HTTPClient: &http.Client{Transport: testTransport},
			}

			opts := cloud.UpdateStackPreviewOpts{
				OrgUUID:        cloud.UUID(tc.orgUUID),
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

func makeAffectedStacks(num int) map[string]*config.Stack {
	affectedStacks := map[string]*config.Stack{}
	for i := 0; i < num; i++ {
		affectedStacks[strconv.Itoa(i)] = &config.Stack{
			Dir:         project.NewPath("/somepath" + strconv.Itoa(i)),
			ID:          uuid.NewString(),
			Name:        "stack" + strconv.Itoa(i),
			Description: "desc of stack" + strconv.Itoa(i),
			Tags:        []string{"tag1", "tag2"},
		}
	}

	return affectedStacks
}

func makeRunContexts(num int, cmd []string) []cloud.RunContext {
	runs := make([]cloud.RunContext, num)
	for i := 0; i < num; i++ {
		runs[i] = cloud.RunContext{
			Stack: &config.Stack{
				Dir:         project.NewPath("/somepath" + strconv.Itoa(i)),
				ID:          uuid.NewString(),
				Name:        "stack" + strconv.Itoa(i),
				Description: "desc of stack" + strconv.Itoa(i),
				Tags:        []string{"tag1", "tag2"},
			},
			Cmd: cmd,
		}
	}

	return runs
}

type previewTransport struct {
	// receivedReqs is a list of all requests that were processed by the transport.
	receivedReqs []*http.Request
}

func (f *previewTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	f.receivedReqs = append(f.receivedReqs, req)

	endpoint := req.Method + " " + req.URL.Path
	switch {
	case strings.HasPrefix(endpoint, "POST /v1/previews"):
		return createPreviewsHandler(req)
	case strings.HasPrefix(endpoint, "PATCH /v1/stack_previews"):
		return updateStackPreviewsHandler(req)
	default:
		return &http.Response{
			StatusCode: http.StatusNotFound,
		}, nil
	}

}

// updateStackPreviewsHandler is a handler for the PATCH /v1/stack_previews endpoint.
func updateStackPreviewsHandler(req *http.Request) (*http.Response, error) {
	var reqParsed cloud.UpdateStackPreviewPayloadRequest
	if err := json.NewDecoder(req.Body).Decode(&reqParsed); err != nil {
		return &http.Response{StatusCode: http.StatusInternalServerError}, nil
	}

	validStatuses := []string{"running", "changed", "unchanged", "failed", "canceled"}
	if !slices.Contains(validStatuses, reqParsed.Status) {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
		}, nil
	}

	return &http.Response{
		StatusCode: http.StatusNoContent,
	}, nil
}

// createPreviewsHandler is a handler for the POST /v1/previews endpoint.
func createPreviewsHandler(req *http.Request) (*http.Response, error) {
	var reqParsed cloud.CreatePreviewPayloadRequest
	if err := json.NewDecoder(req.Body).Decode(&reqParsed); err != nil {
		return &http.Response{StatusCode: http.StatusInternalServerError}, nil
	}

	resp := cloud.CreatePreviewResponse{PreviewID: "1", Stacks: []cloud.ResponsePreviewStack{}}
	for i, s := range reqParsed.Stacks {
		resp.Stacks = append(resp.Stacks, cloud.ResponsePreviewStack{
			MetaID:         s.MetaID,
			StackPreviewID: strconv.Itoa(i),
		})
	}

	respBytes, err := json.Marshal(resp)
	if err != nil {
		return &http.Response{StatusCode: http.StatusInternalServerError}, nil
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(respBytes)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}, nil
}

func (f *previewTransport) httpEndpointsCalled() []string {
	endpoints := make([]string, len(f.receivedReqs))
	for i, r := range f.receivedReqs {
		endpoints[i] = fmt.Sprintf("%s %s", r.Method, r.URL.Path)
	}
	return endpoints
}
