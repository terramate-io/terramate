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
			_, _ = w.Write([]byte("Trailing comma detected in fields parameter"))
			return
		}

		// Return a valid empty response if the request is correct
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"values": []}`))
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

func TestClient_GetPullRequest(t *testing.T) {
	// Setup a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that the URL path is correct
		if !strings.Contains(r.URL.Path, "/repositories/workspace/repo/pullrequests/123") {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Return a valid dummy PR response
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id": 123, "title": "Test PR"}`))
	}))
	defer server.Close()

	client := Client{
		BaseURL:   server.URL,
		Workspace: "workspace",
		RepoSlug:  "repo",
		Token:     "token",
	}

	pr, err := client.GetPullRequest(context.Background(), 123)
	if err != nil {
		t.Fatalf("Expected no error, but got: %v", err)
	}

	if pr.ID != 123 {
		t.Errorf("Expected PR ID 123, got %d", pr.ID)
	}
}

func TestClient_GetPullRequestsByCommit_NoFieldsQuery(t *testing.T) {
	// Setup a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that 'fields' query parameter is NOT present
		if r.URL.Query().Has("fields") {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("Unexpected 'fields' query parameter"))
			return
		}

		// Return a valid empty response
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"values": []}`))
	}))
	defer server.Close()

	client := Client{
		BaseURL:   server.URL,
		Workspace: "workspace",
		RepoSlug:  "repo",
		Token:     "token",
	}

	_, err := client.GetPullRequestsByCommit(context.Background(), "commit-hash")

	if err != nil {
		t.Fatalf("Expected no error, but got: %v", err)
	}
}
