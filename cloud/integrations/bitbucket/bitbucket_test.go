// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package bitbucket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClient_GetPullRequestsByCommit_TrailingComma(t *testing.T) {
	// Setup a mock server that checks for the trailing comma
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that the URL path and query parameters are correct
		if !strings.Contains(r.URL.Path, "/repositories/workspace/repo/commit/commit-hash/pullrequests") {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		fields := r.URL.Query().Get("fields")
		// Assert that 'fields' query parameter does NOT end with a comma
		if strings.HasSuffix(fields, ",") {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Trailing comma detected in fields parameter"))
			return
		}

		// Return a valid empty response if the request is correct
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"values": []}`))
	}))
	defer server.Close()

	client := Client{
		BaseURL:   server.URL,
		Workspace: "workspace",
		RepoSlug:  "repo",
		Token:     "token",
	}

	_, err := client.GetPullRequestsByCommit(context.Background(), "commit-hash")

	// We expect NO error now
	if err != nil {
		t.Fatalf("Expected no error, but got: %v", err)
	}
}
