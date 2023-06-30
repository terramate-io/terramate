package cloud_test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cloud"
)

func TestNormalizeGitURL(t *testing.T) {
	type testcase struct {
		name       string
		raw        string
		normalized string
	}

	tempDir := t.TempDir()

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
			name:       "github ssh url without .git suffix",
			raw:        "git@github.com:terramate-io/terramate.git",
			normalized: "github.com/terramate-io/terramate",
		},
		{
			name:       "https url from any domain",
			raw:        "https://domain/path",
			normalized: "domain/path",
		},
		{
			name:       "https url from ipv4 ip address",
			raw:        "https://192.168.1.169/path",
			normalized: "192.168.1.169/path",
		},
		{
			name:       "ssh url from any domain",
			raw:        "git@example.com:path.git",
			normalized: "example.com/path",
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
			name:       "unrecognized ssh url",
			raw:        "git@github.com:8888:terramate-io/terramate.git",
			normalized: "git@github.com:8888:terramate-io/terramate.git",
		},
		{
			name:       "unrecognized git url with ssh:// prefix",
			raw:        "ssh://git@github.com/terramate-io/terramate.git",
			normalized: "ssh://git@github.com/terramate-io/terramate.git",
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
