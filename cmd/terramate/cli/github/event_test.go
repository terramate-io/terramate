// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package github_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cmd/terramate/cli/github"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/test"
	errtest "github.com/terramate-io/terramate/test/errors"
)

const validGithubEventPath = "./testdata/event_pull_request.json"

func TestGetEventPR(t *testing.T) {
	type want struct {
		err       error
		updatedAt string
		htmlURL   string
		number    int
		title     string
		body      string
		headSHA   string
		draft     bool
	}

	nonExistentEventFile := filepath.Join(test.NonExistingDir(t), "event_pull_request.json")

	type testcase struct {
		name string
		env  map[string]string
		want want
	}

	testcases := []testcase{
		{
			name: "valid event",
			env: map[string]string{
				"GITHUB_EVENT_PATH": validGithubEventPath,
			},
			want: want{
				htmlURL:   "https://github.com/someorg/somerepo/pull/1494",
				number:    1494,
				title:     "envvar",
				body:      "test",
				headSHA:   "ea61b5bd72dec0878ae388b04d76a988439d1e28",
				draft:     false,
				updatedAt: "2024-02-09 12:38:32 +0000 UTC",
			},
		},
		{
			name: "non existent path",
			env: map[string]string{
				"GITHUB_EVENT_PATH": nonExistentEventFile,
			},
			want: want{
				err: os.ErrNotExist,
			},
		},
		{
			name: "missing env var",
			env:  map[string]string{},
			want: want{
				err: errors.E(github.ErrGithubEventPathEnvNotSet),
			},
		},
	}

	// This is needed for tests to have a clean environment when running in GHA
	if err := os.Unsetenv("GITHUB_EVENT_PATH"); err != nil {
		t.Fatal(err)
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			pull, err := github.GetEventPR()
			errtest.Assert(t, err, tc.want.err)

			if tc.want.updatedAt != "" {
				assert.EqualStrings(t, tc.want.updatedAt, pull.GetUpdatedAt().String())
			}

			if tc.want.htmlURL != "" {
				assert.EqualStrings(t, tc.want.htmlURL, pull.GetHTMLURL())
			}

			if tc.want.number != 0 {
				assert.EqualInts(t, tc.want.number, pull.GetNumber())
			}

			if tc.want.title != "" {
				assert.EqualStrings(t, tc.want.title, pull.GetTitle())
			}

			if tc.want.body != "" {
				assert.EqualStrings(t, tc.want.body, pull.GetBody())
			}

			if tc.want.headSHA != "" {
				assert.EqualStrings(t, tc.want.headSHA, pull.GetHead().GetSHA())
			}

			if tc.want.draft != pull.GetDraft() {
				t.Errorf("unexpected draft: want %v, got %v", tc.want.draft, pull.GetDraft())
			}
		})
	}
}
