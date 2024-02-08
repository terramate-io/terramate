// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package testserver

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/terramate-io/terramate/cloud/testserver/cloudstore"
)

// GetCommit returns a commit.
// GitHub API docs: https://docs.github.com/rest/commits/commits#get-a-commit
func GetCommit(store *cloudstore.Data, w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	w.Header().Add("Content-Type", "application/json")
	resp, err := store.GetGithubCommitResponse().MarshalJSON()
	if err != nil {
		http.Error(w, "unable to marshal", http.StatusInternalServerError)
	}
	write(w, resp)
}

// GetPullRequest returns a pull request.
// GitHub API docs: https://docs.github.com/rest/pulls/pulls#get-a-pull-request
func GetPullRequest(store *cloudstore.Data, w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	w.Header().Add("Content-Type", "application/json")
	resp, err := store.GetGithubPullRequestResponse().MarshalJSON()
	if err != nil {
		http.Error(w, "unable to marshal", http.StatusInternalServerError)
	}
	write(w, resp)
}

// ListPullRequestsWithCommit returns a list of pull requests for a commit.
// GitHub API docs: https://docs.github.com/rest/commits/commits#list-pull-requests-associated-with-a-commit
func ListPullRequestsWithCommit(_ *cloudstore.Data, w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	http.Error(w, "ListPullRequestsWithCommit not implemented", http.StatusNotImplemented)
}

// ListReviews returns a list of reviews for a pull request.
// GitHub API docs: https://docs.github.com/rest/pulls/reviews#list-reviews-for-a-pull-request
func ListReviews(_ *cloudstore.Data, w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	http.Error(w, "ListReviews not implemented", http.StatusNotImplemented)
}

// ListCheckRunsForRef returns a list of check runs for a ref.
// GitHub API docs: https://docs.github.com/rest/checks/runs#list-check-runs-for-a-git-reference
func ListCheckRunsForRef(_ *cloudstore.Data, w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	http.Error(w, "ListCheckRunsForRef not implemented", http.StatusNotImplemented)
}

// PullRequestIsMerged returns whether a pull request is merged.
// GitHub API docs: https://docs.github.com/rest/pulls/pulls#check-if-a-pull-request-has-been-merged
func PullRequestIsMerged(_ *cloudstore.Data, w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	http.Error(w, "PullRequestIsMerged not implemented", http.StatusNotImplemented)
}
