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

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/errors"
	errtest "github.com/terramate-io/terramate/test/errors"
)

func TestCloudCustomHTTPClient(t *testing.T) {
	isCalled := false
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isCalled = true
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
			name:       "unauthorized request",
			statusCode: http.StatusUnauthorized,
			want: want{
				err: errors.E(cloud.ErrUnexpectedStatus),
			},
		},
		{
			name:       "unexpected status code",
			statusCode: http.StatusCreated,
			want: want{
				err: errors.E(cloud.ErrUnexpectedStatus),
			},
		},
		{
			name:       "unsupported content-type",
			statusCode: http.StatusOK,
			body:       `[]`,
			headers: http.Header{
				"Content-Type": []string{"application/xml"},
			},
			want: want{
				err: errors.E(cloud.ErrUnexpectedResponseBody),
			},
		},
		{
			name:       "invalid response payload",
			statusCode: http.StatusOK,
			body: `{
					"invalid": 2
			}`,
			want: want{
				err: errors.E(cloud.ErrUnexpectedResponseBody),
			},
		},
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
			s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if len(tc.headers) > 0 {
					for k, v := range tc.headers {
						w.Header().Set(k, v[0])
					}
				} else {
					w.Header().Set("Content-Type", "application/json")
				}
				w.WriteHeader(tc.statusCode)
				_, _ = io.WriteString(w, tc.body)
			}))
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

func credential() cloud.Credential {
	c := &mockCred{}
	return c
}

type mockCred struct{}

func (*mockCred) Token() (string, error) {
	return "I am a token", nil
}
