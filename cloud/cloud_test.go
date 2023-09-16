// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/stack"
	"github.com/terramate-io/terramate/errors"
	errtest "github.com/terramate-io/terramate/test/errors"
)

func TestCloudCustomHTTPClient(t *testing.T) {
	isCalled := false
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isCalled = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, "[]")
	}))
	defer s.Close()

	checkReq := func(client *http.Client, reused bool, assert func(t *testing.T, gotErr error)) {
		isCalled = false
		tmClient := cloud.Client{
			BaseURL:    s.URL,
			HTTPClient: client,
			Credential: credential(),
		}

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		trace := &httptrace.ClientTrace{
			GotConn: func(connInfo httptrace.GotConnInfo) {
				if connInfo.Reused != reused {
					t.Fatal("connection reuse mismatch")
				}
			},
		}

		_, gotErr := tmClient.MemberOrganizations(httptrace.WithClientTrace(ctx, trace))
		assert(t, gotErr)

		if gotErr == nil && !isCalled {
			t.Fatal("cloud.Client.HTTPClient is not being used")
		}
	}

	checkReq(http.DefaultClient, false, func(t *testing.T, gotErr error) {
		if gotErr == nil {
			t.Fatal("should fail because DefaultClient has no valid certificate for the test server")
		}
	})

	// using variable to be clear
	isReused := false

	checkReq(s.Client(), isReused, func(t *testing.T, gotErr error) {
		assert.NoError(t, gotErr)
	})

	// previous connection must be reused
	isReused = true
	checkReq(s.Client(), isReused, func(t *testing.T, gotErr error) {
		assert.NoError(t, gotErr)
	})
}

func TestCommonAPIFailCases(t *testing.T) {
	type testcase struct {
		name       string
		statusCode int
		body       string
		headers    http.Header
		err        error
	}

	for _, tc := range []testcase{
		{
			name:       "unauthorized request",
			statusCode: http.StatusUnauthorized,
			err:        errors.E(cloud.ErrUnexpectedStatus),
		},
		{
			name:       "unexpected status code",
			statusCode: http.StatusInternalServerError,
			err:        errors.E(cloud.ErrUnexpectedStatus),
		},
		{
			name:       "unsupported content-type",
			statusCode: http.StatusOK,
			body:       `[]`,
			headers: http.Header{
				"Content-Type": []string{"application/xml"},
			},

			err: errors.E(cloud.ErrUnexpectedResponseBody),
		},
		{
			name:       "invalid response payload",
			statusCode: http.StatusOK,
			body: `{
					"stacks": 2
			}`,
			err: errors.E(cloud.ErrUnexpectedResponseBody),
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := newTestServer(tc.statusCode, tc.body, tc.headers)
			defer s.Close()

			sdk := cloud.Client{
				BaseURL:    s.URL,
				HTTPClient: s.Client(),
				Credential: credential(),
			}

			// /v1/users
			func() {
				const timeout = 3 * time.Second
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				defer cancel()

				_, err := sdk.Users(ctx)
				errtest.Assert(t, err, tc.err)
			}()

			// /v1/memberships
			func() {
				const timeout = 3 * time.Second
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				defer cancel()

				_, err := sdk.MemberOrganizations(ctx)
				errtest.Assert(t, err, tc.err)
			}()

			// /v1/stacks
			func() {
				const timeout = 3 * time.Second
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				defer cancel()

				_, err := sdk.Stacks(ctx, "e4c81294-dcf8-45e2-ba95-25f96514a61b", stack.NoFilter)
				errtest.Assert(t, err, tc.err)
			}()
		})
	}
}

func TestCloudMemberOrganizations(t *testing.T) {
	type want struct {
		orgs cloud.MemberOrganizations
		err  error
	}
	type testcase struct {
		name       string
		statusCode int
		body       string
		headers    http.Header
		want       want
	}

	for _, tc := range []testcase{
		{
			name:       "invalid organization object",
			statusCode: http.StatusOK,
			body: `[
				{}
			]`,
			want: want{
				err: errors.E(cloud.ErrUnexpectedResponseBody),
			},
		},
		{
			name:       "invalid organization object -- missing uuid field",
			statusCode: http.StatusOK,
			body: `[
				{
					"org_name": "terramate-io"
				}
			]`,
			want: want{
				err: errors.E(cloud.ErrUnexpectedResponseBody),
			},
		},
		{
			name:       "valid simple request",
			statusCode: http.StatusOK,
			body: `[
				{
					"org_name": "terramate-io",
					"org_uuid": "0000-0000-0000-0000"
				}
			]`,
			want: want{
				orgs: cloud.MemberOrganizations{
					cloud.MemberOrganization{
						Name: "terramate-io",
						UUID: "0000-0000-0000-0000",
					},
				},
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := newTestServer(tc.statusCode, tc.body, tc.headers)
			defer s.Close()

			sdk := cloud.Client{
				BaseURL:    s.URL,
				HTTPClient: s.Client(),
				Credential: credential(),
			}

			const timeout = 3 * time.Second
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			orgs, err := sdk.MemberOrganizations(ctx)
			errtest.Assert(t, err, tc.want.err)
			if err != nil {
				return
			}

			assert := assert.New(t, assert.Fatal, "asserting orgs")
			assert.Partial(orgs, tc.want.orgs)
		})
	}
}

func TestCloudStacks(t *testing.T) {
	type want struct {
		stacks cloud.StacksResponse
		err    error
	}
	type testcase struct {
		name       string
		org        string
		filter     stack.FilterStatus
		statusCode int
		body       string
		headers    http.Header
		want       want
	}

	for _, tc := range []testcase{
		{
			name:       "non-existent organization returns empty stacks list",
			org:        "df580ab4-b20d-4b1d-afc3-3bdccc56491b",
			statusCode: http.StatusOK,
			body: `{
				"stacks": []
			}`,
			want: want{
				stacks: cloud.StacksResponse{
					Stacks: []cloud.StackResponse{},
				},
			},
		},
		{
			name:       "stack missing MetaID",
			org:        "df580ab4-b20d-4b1d-afc3-3bdccc56491b",
			statusCode: http.StatusOK,
			body: `{
				"stacks": [
					{
						"stack_id": 666,
						"repository": "github.com/terramate-io/terramate",
						"path": "/docs",
						"meta_name": "documentation",
						"meta_description": "terramate documentation",
						"meta_tags": [
						  "docs"
						],
						"status": "ok",
						"created_at": "2023-08-02T08:26:39.03748Z",
						"updated_at": "2023-08-02T08:26:39.03748Z",
						"seen_at": "2023-08-11T09:54:50.70824Z"
					}
				]
			}`,
			want: want{
				err: errors.E(cloud.ErrUnexpectedResponseBody),
			},
		},
		{
			name:       "stack missing status",
			org:        "df580ab4-b20d-4b1d-afc3-3bdccc56491b",
			statusCode: http.StatusOK,
			body: `{
				"stacks": [
					{
						"stack_id": 666,
						"repository": "github.com/terramate-io/terramate",
						"path": "/docs",
						"meta_id": "docs"
						"meta_name": "documentation",
						"meta_description": "terramate documentation",
						"meta_tags": [
						  "docs"
						],
						"created_at": "2023-08-02T08:26:39.03748Z",
						"updated_at": "2023-08-02T08:26:39.03748Z",
						"seen_at": "2023-08-11T09:54:50.70824Z"
					}
				]
			}`,
			want: want{
				err: errors.E(cloud.ErrUnexpectedResponseBody),
			},
		},
		{
			name:       "stack with unrecognized status",
			org:        "df580ab4-b20d-4b1d-afc3-3bdccc56491b",
			statusCode: http.StatusOK,
			body: `{
				"stacks": [
					{
						"stack_id": 666,
						"repository": "github.com/terramate-io/terramate",
						"path": "/docs",
						"meta_id": "0aef0c2b-3314-4097-a7e5-3d6d03cb4604",
						"meta_name": "documentation",
						"meta_description": "terramate documentation",
						"meta_tags": [
						  "docs"
						],
						"status": "unrecognized",
						"created_at": "2023-08-02T08:26:39.03748Z",
						"updated_at": "2023-08-02T08:26:39.03748Z",
						"seen_at": "2023-08-11T09:54:50.70824Z"
					}
				]
			}`,
			want: want{
				stacks: cloud.StacksResponse{
					Stacks: []cloud.StackResponse{
						{
							ID: 666,
							Stack: cloud.Stack{
								Repository:      "github.com/terramate-io/terramate",
								Path:            "/docs",
								MetaID:          "0aef0c2b-3314-4097-a7e5-3d6d03cb4604",
								MetaName:        "documentation",
								MetaDescription: "terramate documentation",
								MetaTags:        []string{"docs"},
							},
							Status: stack.Unrecognized,
						},
					},
				},
			},
		},
		{
			name:       "stack with no repository",
			org:        "df580ab4-b20d-4b1d-afc3-3bdccc56491b",
			statusCode: http.StatusOK,
			body: `{
				"stacks": [
					{
						"stack_id": 666,
						"path": "/docs",
						"meta_id": "0aef0c2b-3314-4097-a7e5-3d6d03cb4604",
						"meta_name": "documentation",
						"meta_description": "terramate documentation",
						"meta_tags": [
						  "docs"
						],
						"status": "ok",
						"created_at": "2023-08-02T08:26:39.03748Z",
						"updated_at": "2023-08-02T08:26:39.03748Z",
						"seen_at": "2023-08-11T09:54:50.70824Z"
					}
				]
			}`,
			want: want{
				err: errors.E(cloud.ErrUnexpectedResponseBody),
			},
		},
		{
			name:       "valid object",
			org:        "df580ab4-b20d-4b1d-afc3-3bdccc56491b",
			statusCode: http.StatusOK,
			body: `{
				"stacks": [
					{
						"stack_id": 666,
						"repository": "github.com/terramate-io/terramate",
						"path": "/docs",
						"meta_id": "0aef0c2b-3314-4097-a7e5-3d6d03cb4604",
						"meta_name": "documentation",
						"meta_description": "terramate documentation",
						"meta_tags": [
						  "docs"
						],
						"status": "ok",
						"created_at": "2023-08-02T08:26:39.03748Z",
						"updated_at": "2023-08-02T08:26:39.03748Z",
						"seen_at": "2023-08-11T09:54:50.70824Z"
					},
					{
						"stack_id": 667,
						"repository": "github.com/terramate-io/terramate",
						"path": "/",
						"meta_id": "4ff324cd-f338-4526-8bcb-28ec33bbaeea",
						"meta_name": "terramate",
						"meta_description": "terramate source code",
						"meta_tags": [
						  "golang"
						],
						"status": "ok",
						"created_at": "2023-08-02T08:26:39.03748Z",
						"updated_at": "2023-08-02T08:26:39.03748Z",
						"seen_at": "2023-08-11T09:54:50.70824Z"
					},
					{
						"stack_id": 668,
						"repository": "github.com/terramate-io/terramate",
						"path": "/_testdata/example-stack",
						"meta_id": "terramate-example-stack",
						"meta_name": "test-stacks",
						"meta_description": "Used in terramate tests",
						"meta_tags": [
						  "test"
						],
						"status": "ok",
						"created_at": "2023-08-02T08:26:39.03748Z",
						"updated_at": "2023-08-02T08:26:39.03748Z",
						"seen_at": "2023-08-11T09:54:50.70824Z"
					}
				]
			}`,
			want: want{
				stacks: cloud.StacksResponse{
					Stacks: []cloud.StackResponse{
						{
							ID: 666,
							Stack: cloud.Stack{
								Repository:      "github.com/terramate-io/terramate",
								Path:            "/docs",
								MetaID:          "0aef0c2b-3314-4097-a7e5-3d6d03cb4604",
								MetaName:        "documentation",
								MetaDescription: "terramate documentation",
								MetaTags:        []string{"docs"},
							},
							Status: stack.OK,
						},
						{
							ID: 667,
							Stack: cloud.Stack{
								Repository:      "github.com/terramate-io/terramate",
								Path:            "/",
								MetaID:          "4ff324cd-f338-4526-8bcb-28ec33bbaeea",
								MetaName:        "terramate",
								MetaDescription: "terramate source code",
								MetaTags:        []string{"golang"},
							},
							Status: stack.OK,
						},
						{
							ID: 668,
							Stack: cloud.Stack{
								Repository:      "github.com/terramate-io/terramate",
								Path:            "/_testdata/example-stack",
								MetaID:          "terramate-example-stack",
								MetaName:        "test-stacks",
								MetaDescription: "Used in terramate tests",
								MetaTags:        []string{"test"},
							},
							Status: stack.OK},
					},
				},
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := newTestServer(tc.statusCode, tc.body, tc.headers)
			defer s.Close()

			sdk := cloud.Client{
				BaseURL:    s.URL,
				HTTPClient: s.Client(),
				Credential: credential(),
			}

			const timeout = 3 * time.Second
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			stacksResp, err := sdk.Stacks(ctx, tc.org, tc.filter)
			errtest.Assert(t, err, tc.want.err)
			if err != nil {
				return
			}

			if diff := cmp.Diff(stacksResp, tc.want.stacks, cmpopts.IgnoreTypes(time.Time{})); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func newTestServer(statusCode int, body string, headers http.Header) *httptest.Server {
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(headers) > 0 {
			for k, v := range headers {
				w.Header().Set(k, v[0])
			}
		} else {
			w.Header().Set("Content-Type", "application/json")
		}
		w.WriteHeader(statusCode)
		_, _ = io.WriteString(w, body)
	}))
}

func credential() cloud.Credential {
	c := &mockCred{}
	return c
}

type mockCred struct{}

func (*mockCred) Token() (string, error) {
	return "I am a token", nil
}
