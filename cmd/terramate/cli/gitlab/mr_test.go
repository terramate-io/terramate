// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package gitlab_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cmd/terramate/cli/gitlab"
)

func TestGitlabLatestMRForCommit(t *testing.T) {
	const (
		ns        = "terramate-io"
		project   = "terramate"
		projectID = 1
		sha       = "deadbeefdeadbeefdeadbeefdeadbeef"
	)

	mux, client := setup(t, projectID)
	endpoint := fmt.Sprintf("/api/v4/projects/%d/repository/commits/%s/merge_requests", projectID, sha)
	responsePayload := genMockedResponseMR(ns, project, sha)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// catch all handler
		checkMethod(t, "GET", r)
		if r.URL.Path != endpoint {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fmt.Fprint(w, responsePayload)
	})

	got, found, err := client.MRForCommit(context.Background(), sha)
	assert.NoError(t, err)
	if !found {
		t.Fatal("expected 1 MR")
	}

	var expected gitlab.MRs
	assert.NoError(t, json.Unmarshal([]byte(responsePayload), &expected))

	if diff := cmp.Diff(got, expected[0]); diff != "" {
		t.Fatalf("unexpected response: %s", diff)
	}
}

func genMockedResponseMR(ns, prj, sha string) string {
	return fmt.Sprintf(`[{
		"id": 1,
		"iid": 1,
		"project_id": 3,
		"title": "Terramate is awesome",
		"description": "Some amazing feature",
		"state": "opened",
		"target_branch": "main",
		"source_branch": "feature",
		"upvotes": 100,
		"downvotes": 1,
		"author": {
			"id": 1337,
			"name": "leet",
			"username": "leet",
			"state": "active",
			"avatar_url": "https://some.place/image.png",
			"web_url" : "https://gitlab.com/leet"
		},
		"assignee": {
			"id": 1337,
			"name": "leet",
			"username": "leet",
			"state": "active",
			"avatar_url": "https://some.place/image.png",
			"web_url" : "https://gitlab.com/leet"
		},
		"draft": true,
		"work_in_progress": true,
		"sha": %q,
		"web_url": "http://gitlab.com/%s/merge_requests/1"
	}]`, sha, url.QueryEscape(ns+"/"+prj))
}

func checkMethod(t *testing.T, method string, r *http.Request) {
	t.Helper()
	assert.EqualStrings(t, method, r.Method)
}

func setup(t *testing.T, projectID int) (*http.ServeMux, *gitlab.Client) {
	mux := http.NewServeMux()

	// server is a test HTTP server used to provide mock API responses.
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return mux, &gitlab.Client{
		BaseURL:    server.URL + "/api/v4",
		ProjectID:  projectID,
		HTTPClient: server.Client(),
	}
}
