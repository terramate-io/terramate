// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package gitlab_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cmd/terramate/cli/gitlab"
)

// small gitlab runners sometimes froze by some seconds.
const apiTestTimeout = 5 * time.Second

func TestGitlabSmokeTestMR(t *testing.T) {
	if !isSmokeTestEnabled() {
		t.Skip("skipping gitlab smoke tests")
	}

	token := os.Getenv("TM_TEST_GITLAB_TOKEN")
	if token == "" {
		t.Fatal("Gitlab CI is misconfigured: missing TM_TEST_GITLAB_TOKEN environment variable")
	}

	client := gitlab.Client{
		Token:   token,
		Group:   "terramate-io",
		Project: "test",
	}

	ctx, cancel := context.WithTimeout(context.Background(), apiTestTimeout)
	defer cancel()

	// all main commits MUST have a merge
	mr, found, err := client.MRForCommit(ctx, "2b2383915aa09510b9ef77a48617dff8c2ad17ad")
	assert.NoError(t, err)
	if !found {
		t.Fatalf("no MR found")
	}
	expected := gitlab.MR{
		ID:           297900704,
		IID:          1,
		ProjectID:    56867221,
		Title:        "test: check GitLab OIDC info.",
		Description:  "Signed-off-by: Tiago Natel <t.nateldemoura@gmail.com>",
		State:        "merged",
		CreatedAt:    "2024-04-24T00:30:37.308Z",
		UpdatedAt:    "2024-06-20T11:36:22.029Z",
		TargetBranch: "main",
		SourceBranch: "i4k-test-oidc",
		SHA:          "dfca2ad01e8c2bae98d221a854dd3a7f3dd7e65c",
		MergeStatus:  "can_be_merged",
		WebURL:       "https://gitlab.com/terramate-io/test/-/merge_requests/1",
		Labels:       []string{"tests"},
	}
	if diff := cmp.Diff(expected, mr, cmpopts.IgnoreFields(gitlab.MR{}, "Author")); diff != "" {
		t.Fatal(diff)
	}
}

func isSmokeTestEnabled() bool {
	return isEnvTrue("CI") && isEnvTrue("GITLAB_CI") &&
		isEnvFalse("TM_TEST_SKIP_GITLAB_SMOKETEST")
}

func isEnvTrue(name string) bool {
	value := os.Getenv(name)
	return value == "true" || value == "1"
}

func isEnvFalse(name string) bool {
	value := os.Getenv(name)
	return value == "" || value == "false" || value == "0"
}
