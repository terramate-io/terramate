// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package ci_test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/ci"
)

func TestDetectPlatformFromEnv(t *testing.T) {
	tests := map[string]ci.PlatformType{
		"GITHUB_ACTIONS":         ci.PlatformGithub,
		"GITLAB_CI":              ci.PlatformGitlab,
		"BITBUCKET_BUILD_NUMBER": ci.PlatformBitBucket,
		"TF_BUILD":               ci.PlatformAzureDevops,
		"CI":                     ci.PlatformGenericCI,
	}

	for k, want := range tests {
		t.Run(k, func(t *testing.T) {
			for k2 := range tests {
				if k == k2 {
					t.Setenv(k, "1")
				} else {
					t.Setenv(k2, "")
				}
			}
			platform := ci.DetectPlatformFromEnv()
			assert.EqualInts(t, int(want), int(platform))
		})
	}
}
