// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package bitbucket

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClient_GetPullRequestsForCommit(t *testing.T) {
	// Setup a mock server that checks for the query parameter
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that the URL path and query parameters are correct
		if !strings.Contains(r.URL.Path, "/repositories/workspace/repo/pullrequests") {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		q := r.URL.Query().Get("q")
		expectedQuery := `(source.branch.name="main" OR destination.branch.name="main") AND (state="OPEN" OR state="MERGED" OR state="DECLINED")`
		if q != expectedQuery {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprintf(w, "Unexpected q parameter: got %s, want %s", q, expectedQuery)
			return
		}

		if r.URL.Query().Get("sort") != "-updated_on" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("Unexpected sort parameter"))
			return
		}

		if r.URL.Query().Get("pagelen") != "50" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("Unexpected pagelen parameter"))
			return
		}

		// Return a response with multiple PRs to test client-side filtering
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"values": [
				{
					"id": 1,
					"source": { "commit": { "hash": "target-commit-hash" } },
					"merge_commit": { "hash": "other-hash" }
				},
				{
					"id": 2,
					"source": { "commit": { "hash": "other-hash" } },
					"merge_commit": { "hash": "target-commit-hash" }
				},
				{
					"id": 3,
					"source": { "commit": { "hash": "non-matching-hash" } },
					"merge_commit": { "hash": "non-matching-hash" }
				}
			]
		}`))
	}))
	defer server.Close()

	client := Client{
		BaseURL:   server.URL,
		Workspace: "workspace",
		RepoSlug:  "repo",
		Token:     "token",
	}

	prs, err := client.GetPullRequestsForCommit(context.Background(), "target-commit-hash", "main")

	// We expect NO error
	if err != nil {
		t.Fatalf("Expected no error, but got: %v", err)
	}

	// We expect 2 matching PRs (id 1 and 2)
	if len(prs) != 2 {
		t.Errorf("Expected 2 matching PRs, got %d", len(prs))
	}
}

func TestClient_GetPullRequestsForCommit_Pagination(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/repositories/workspace/repo/pullrequests") {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		page := r.URL.Query().Get("page")

		if page == "" || page == "1" {
			w.WriteHeader(http.StatusOK)
			// Return first page with next link pointing to this same server
			// We iterate through pages.
			nextURL := fmt.Sprintf("%s/repositories/workspace/repo/pullrequests?page=2", server.URL)

			// We MUST escape JSON string if needed, but URL is safe here.
			response := fmt.Sprintf(`{
				"next": "%s",
				"values": [
					{
						"id": 1,
						"source": { "commit": { "hash": "other-hash" } },
						"merge_commit": { "hash": "other-hash" }
					}
				]
			}`, nextURL)
			_, _ = w.Write([]byte(response))
			return
		}

		if page == "2" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"values": [
					{
						"id": 2,
						"source": { "commit": { "hash": "target-commit-hash" } },
						"merge_commit": { "hash": "other-hash" }
					}
				]
			}`))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := Client{
		BaseURL:   server.URL,
		Workspace: "workspace",
		RepoSlug:  "repo",
		Token:     "token",
	}

	// We pass a dummy branch since we are testing pagination flow mainly
	prs, err := client.GetPullRequestsForCommit(context.Background(), "target-commit-hash", "target-branch")

	if err != nil {
		t.Fatalf("Expected no error, but got: %v", err)
	}

	if len(prs) != 1 {
		t.Errorf("Expected 1 match from page 2, got %d", len(prs))
	}
	if len(prs) > 0 && prs[0].ID != 2 {
		t.Errorf("Expected PR ID 2, got %d", prs[0].ID)
	}
}

func TestClient_GetPullRequestsForCommit_SpecialChars(t *testing.T) {
	// Setup a mock server that checks for the correctly escaped query parameter
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		// internal/quote"branch -> internal/quote\"branch
		// We expect the query to contain the escaped version
		expectedPart := `source.branch.name="internal/quote\"branch"`
		if !strings.Contains(q, expectedPart) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprintf(w, "Expected query to contain %s, got %s", expectedPart, q)
			return
		}

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

	// Use a branch name with a double quote
	_, err := client.GetPullRequestsForCommit(context.Background(), "commit-hash", `internal/quote"branch`)
	if err != nil {
		t.Fatalf("Expected no error, but got: %v", err)
	}
}

func TestClient_GetPullRequestsForCommit_EmptyBranch(t *testing.T) {
	// Setup a mock server that checks for the legacy URL
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Expect call to /repositories/workspace/repo/commit/commit-hash/pullrequests
		if !strings.Contains(r.URL.Path, "/commit/commit-hash/pullrequests") {
			w.WriteHeader(http.StatusNotFound)
			return
		}

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

	// Pass empty branch
	_, err := client.GetPullRequestsForCommit(context.Background(), "commit-hash", "")
	if err != nil {
		t.Fatalf("Expected no error, but got: %v", err)
	}
}

func TestClient_GetPullRequestsForCommit_NoFieldsQuery(t *testing.T) {
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

	_, err := client.GetPullRequestsForCommit(context.Background(), "commit-hash", "main")

	if err != nil {
		t.Fatalf("Expected no error, but got: %v", err)
	}
}
