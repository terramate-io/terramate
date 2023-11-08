// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud_test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/test"
)

func TestNormalizeGitURL(t *testing.T) {
	t.Parallel()
	type testcase struct {
		name       string
		raw        string
		normalized string
	}

	tempDir := test.TempDir(t)

	for _, tc := range []testcase{
		{
			name:       "basic github https url",
			raw:        "https://github.com/terramate-io/terramate.git",
			normalized: "github.com/terramate-io/terramate",
		},
		{
			name:       "github https url without .git suffix",
			raw:        "https://github.com/terramate-io/terramate",
			normalized: "github.com/terramate-io/terramate",
		},
		{
			name:       "basic github ssh url",
			raw:        "git@github.com:terramate-io/terramate.git",
			normalized: "github.com/terramate-io/terramate",
		},
		{
			name:       "basic gitlab ssh url",
			raw:        "git@gitlab.com:terramate-io/terramate.git",
			normalized: "gitlab.com/terramate-io/terramate",
		},
		{
			name:       "github ssh url without .git suffix",
			raw:        "git@github.com:terramate-io/terramate.git",
			normalized: "github.com/terramate-io/terramate",
		},
		{
			name:       "gitlab ssh url without .git suffix",
			raw:        "git@gitlab.com:terramate-io/terramate.git",
			normalized: "gitlab.com/terramate-io/terramate",
		},
		{
			name:       "malformed owner/repo returns raw (domain)",
			raw:        "https://example.com/path",
			normalized: "https://example.com/path",
		},
		{
			name:       "malformed owner/repo returns raw (ip)",
			raw:        "https://192.168.1.169/path",
			normalized: "https://192.168.1.169/path",
		},
		{
			name:       "ssh url from any domain",
			raw:        "git@example.com:owner/path.git",
			normalized: "example.com/owner/path",
		},
		{
			name:       "filesystem path returns as local",
			raw:        tempDir,
			normalized: "local",
		},
		{
			name:       "unrecognized url return as is",
			raw:        "something else",
			normalized: "something else",
		},
		{
			name:       "unrecognized ssh url - THIS IS A BUG IN THE go-gh library",
			raw:        "git@github.com:8888:terramate-io/terramate.git",
			normalized: "github.com/8888:terramate-io/terramate",
		},
		{
			name:       "github url with ssh:// prefix",
			raw:        "ssh://git@github.com/terramate-io/terramate.git",
			normalized: "github.com/terramate-io/terramate",
		},
		{
			name:       "gitlab url with ssh:// prefix",
			raw:        "ssh://git@gitlab.com/terramate-io/terramate.git",
			normalized: "gitlab.com/terramate-io/terramate",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			assert.EqualStrings(t,
				tc.normalized,
				cloud.NormalizeGitURI(tc.raw),
				"git url normalization mismatch",
			)
		})
	}
}
