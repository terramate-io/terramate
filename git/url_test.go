// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package git_test

import (
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/git"
	"github.com/terramate-io/terramate/test"
	errtest "github.com/terramate-io/terramate/test/errors"
)

func TestNormalizeGitURL(t *testing.T) {
	t.Parallel()
	type testcase struct {
		name       string
		raw        string
		normalized git.Repository
		want       error
	}

	tempDir := test.TempDir(t)

	for _, tc := range []testcase{
		{
			name: "basic github https url",
			raw:  "https://github.com/terramate-io/terramate.git",
			normalized: git.Repository{
				RawURL: "https://github.com/terramate-io/terramate.git",
				Host:   "github.com",
				Repo:   "github.com/terramate-io/terramate",
				Owner:  "terramate-io",
				Name:   "terramate",
			},
		},
		{
			name: "gitlab url with subgroups",
			raw:  "https://gitlab.com/acme/team/my-repo.git",
			normalized: git.Repository{
				RawURL: "https://gitlab.com/acme/team/my-repo.git",
				Host:   "gitlab.com",
				Repo:   "gitlab.com/acme/team/my-repo",
				Owner:  "acme",
				Name:   "team/my-repo",
			},
		},
		{
			name: "github https url without .git suffix",
			raw:  "https://github.com/terramate-io/terramate",
			normalized: git.Repository{
				RawURL: "https://github.com/terramate-io/terramate",
				Repo:   "github.com/terramate-io/terramate",
				Host:   "github.com",
				Owner:  "terramate-io",
				Name:   "terramate",
			},
		},
		{
			name: "gitlab https url without .git suffix",
			raw:  "https://gitlab.com/acme/team/my-repo",
			normalized: git.Repository{
				RawURL: "https://gitlab.com/acme/team/my-repo",
				Host:   "gitlab.com",
				Repo:   "gitlab.com/acme/team/my-repo",
				Owner:  "acme",
				Name:   "team/my-repo",
			},
		},
		{
			name: "basic github ssh url",
			raw:  "git@github.com:terramate-io/terramate.git",
			normalized: git.Repository{
				RawURL: "git@github.com:terramate-io/terramate.git",
				Repo:   "github.com/terramate-io/terramate",
				Host:   "github.com",
				Owner:  "terramate-io",
				Name:   "terramate",
			},
		},
		{
			name: "basic gitlab ssh url",
			raw:  "git@gitlab.com:terramate-io/terramate.git",
			normalized: git.Repository{
				RawURL: "git@gitlab.com:terramate-io/terramate.git",
				Repo:   "gitlab.com/terramate-io/terramate",
				Host:   "gitlab.com",
				Owner:  "terramate-io",
				Name:   "terramate",
			},
		},
		{
			name: "github ssh url without .git suffix",
			raw:  "git@github.com:terramate-io/terramate.git",
			normalized: git.Repository{
				RawURL: "git@github.com:terramate-io/terramate.git",
				Repo:   "github.com/terramate-io/terramate",
				Host:   "github.com",
				Owner:  "terramate-io",
				Name:   "terramate",
			},
		},

		{
			name: "gitlab ssh url without .git suffix",
			raw:  "git@gitlab.com:terramate-io/terramate.git",
			normalized: git.Repository{
				RawURL: "git@gitlab.com:terramate-io/terramate.git",
				Repo:   "gitlab.com/terramate-io/terramate",
				Host:   "gitlab.com",
				Owner:  "terramate-io",
				Name:   "terramate",
			},
		},
		{
			name: "no owner",
			raw:  "https://example.com/path",
			normalized: git.Repository{
				RawURL: "https://example.com/path",
				Repo:   "example.com/path",
				Host:   "example.com",
				Name:   "path",
			},
		},
		{
			name: "IP with no owner",
			raw:  "https://192.168.1.169/path",
			normalized: git.Repository{
				RawURL: "https://192.168.1.169/path",
				Repo:   "192.168.1.169/path",
				Host:   "192.168.1.169",
				Name:   "path",
			},
		},
		{
			name: "ssh url from any domain",
			raw:  "git@example.com:owner/path.git",
			normalized: git.Repository{
				RawURL: "git@example.com:owner/path.git",
				Repo:   "example.com/owner/path",
				Host:   "example.com",
				Owner:  "owner",
				Name:   "path",
			},
		},
		{
			name: "filesystem path returns as local",
			raw:  tempDir,
			normalized: git.Repository{
				RawURL: tempDir,
				Host:   "local",
			},
		},
		{
			name: "no URL and no path gives an error",
			raw:  "something else",
			want: errors.E(git.ErrInvalidGitURL),
		},
		{
			name: "vcs provider URL with port",
			raw:  "git@github.com:8888:terramate-io/terramate.git",
			normalized: git.Repository{
				RawURL: "git@github.com:8888:terramate-io/terramate.git",
				Repo:   "github.com:8888/terramate-io/terramate",
				Host:   "github.com:8888",
				Owner:  "terramate-io",
				Name:   "terramate",
			},
		},
		{
			name: "github url with ssh:// prefix",
			raw:  "ssh://git@github.com/terramate-io/terramate.git",
			normalized: git.Repository{
				RawURL: "ssh://git@github.com/terramate-io/terramate.git",
				Repo:   "github.com/terramate-io/terramate",
				Host:   "github.com",
				Owner:  "terramate-io",
				Name:   "terramate",
			},
		},
		{
			name: "gitlab url with ssh:// prefix",
			raw:  "ssh://git@gitlab.com/terramate-io/terramate.git",
			normalized: git.Repository{
				RawURL: "ssh://git@gitlab.com/terramate-io/terramate.git",
				Repo:   "gitlab.com/terramate-io/terramate",
				Host:   "gitlab.com",
				Owner:  "terramate-io",
				Name:   "terramate",
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			r, err := git.NormalizeGitURI(tc.raw)
			errtest.Assert(t, err, tc.want)
			if err != nil {
				return
			}

			if diff := cmp.Diff(tc.normalized, r); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestIsURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{
			name: "scp-like",
			url:  "git@example.com:owner/repo",
			want: true,
		},
		{
			name: "scp-like with no user",
			url:  "example.com:owner/repo",
			want: false,
		},
		{
			name: "ssh",
			url:  "ssh://git@example.com/owner/repo",
			want: true,
		},
		{
			name: "git",
			url:  "git://example.com/owner/repo",
			want: true,
		},
		{
			name: "git with extension",
			url:  "git://example.com/owner/repo.git",
			want: true,
		},
		{
			name: "git+ssh",
			url:  "git+ssh://git@example.com/owner/repo.git",
			want: true,
		},
		{
			name: "git+https",
			url:  "git+https://example.com/owner/repo.git",
			want: true,
		},
		{
			name: "http",
			url:  "http://example.com/owner/repo.git",
			want: true,
		},
		{
			name: "https",
			url:  "https://example.com/owner/repo.git",
			want: true,
		},
		{
			name: "no protocol",
			url:  "example.com/owner/repo",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := git.IsURL(tt.url); tt.want != got {
				t.Fatalf("wanted[%t] but got[%t] for %s", tt.want, got, tt.url)
			}
		})
	}
}

func TestParseURL(t *testing.T) {
	type url struct {
		Scheme string
		User   string
		Host   string
		Path   string
	}

	tests := []struct {
		name    string
		url     string
		want    url
		wantErr bool
	}{
		{
			name: "HTTPS",
			url:  "https://example.com/owner/repo.git",
			want: url{
				Scheme: "https",
				User:   "",
				Host:   "example.com",
				Path:   "/owner/repo.git",
			},
		},
		{
			name: "HTTP",
			url:  "http://example.com/owner/repo.git",
			want: url{
				Scheme: "http",
				User:   "",
				Host:   "example.com",
				Path:   "/owner/repo.git",
			},
		},
		{
			name: "git",
			url:  "git://example.com/owner/repo.git",
			want: url{
				Scheme: "git",
				User:   "",
				Host:   "example.com",
				Path:   "/owner/repo.git",
			},
		},
		{
			name: "ssh",
			url:  "ssh://git@example.com/owner/repo.git",
			want: url{
				Scheme: "ssh",
				User:   "git",
				Host:   "example.com",
				Path:   "/owner/repo.git",
			},
		},
		{
			name: "ssh with port",
			url:  "ssh://git@example.com:443/owner/repo.git",
			want: url{
				Scheme: "ssh",
				User:   "git",
				Host:   "example.com:443",
				Path:   "/owner/repo.git",
			},
		},
		{
			name: "git+ssh",
			url:  "git+ssh://example.com/owner/repo.git",
			want: url{
				Scheme: "ssh",
				User:   "",
				Host:   "example.com",
				Path:   "/owner/repo.git",
			},
		},
		{
			name: "git+https",
			url:  "git+https://example.com/owner/repo.git",
			want: url{
				Scheme: "https",
				User:   "",
				Host:   "example.com",
				Path:   "/owner/repo.git",
			},
		},
		{
			name: "scp-like",
			url:  "git@example.com:owner/repo.git",
			want: url{
				Scheme: "ssh",
				User:   "git",
				Host:   "example.com",
				Path:   "/owner/repo.git",
			},
		},
		{
			name: "scp-like, leading slash",
			url:  "git@example.com:/owner/repo.git",
			want: url{
				Scheme: "ssh",
				User:   "git",
				Host:   "example.com",
				Path:   "/owner/repo.git",
			},
		},
		{
			name: "file protocol",
			url:  "file:///example.com/owner/repo.git",
			want: url{
				Scheme: "file",
				User:   "",
				Host:   "",
				Path:   "/example.com/owner/repo.git",
			},
		},
		{
			name: "file path",
			url:  "/example.com/owner/repo.git",
			want: url{
				Scheme: "",
				User:   "",
				Host:   "",
				Path:   "/example.com/owner/repo.git",
			},
		},
		{
			name: "Windows file path",
			url:  "C:\\example.com\\owner\\repo.git",
			want: url{
				Scheme: "c",
				User:   "",
				Host:   "",
				Path:   "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := git.ParseURL(tt.url)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.EqualStrings(t, tt.want.Scheme, u.Scheme)
			assert.EqualStrings(t, tt.want.User, u.User.Username())
			assert.EqualStrings(t, tt.want.Host, u.Host)
			assert.EqualStrings(t, tt.want.Path, u.Path)
		})
	}
}

func TestRepoInfoFromURL(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantHost  string
		wantOwner string
		wantRepo  string
		wantErr   error
	}{
		{
			name:      "github.com URL",
			input:     "https://github.com/monalisa/octo-cat.git",
			wantHost:  "github.com",
			wantOwner: "monalisa",
			wantRepo:  "octo-cat",
		},
		{
			name:      "github.com URL with trailing slash",
			input:     "https://github.com/monalisa/octo-cat/",
			wantHost:  "github.com",
			wantOwner: "monalisa",
			wantRepo:  "octo-cat",
		},
		{
			name:      "www.github.com URL",
			input:     "http://www.GITHUB.com/monalisa/octo-cat.git",
			wantHost:  "github.com",
			wantOwner: "monalisa",
			wantRepo:  "octo-cat",
		},
		{
			name:      "many path components - changed for Gitlab support",
			input:     "https://github.com/monalisa/octo-cat/pulls",
			wantOwner: "monalisa",
			wantRepo:  "octo-cat/pulls",
			wantHost:  "github.com",
		},
		{
			name:      "non-GitHub hostname",
			input:     "https://example.com/one/two",
			wantHost:  "example.com",
			wantOwner: "one",
			wantRepo:  "two",
		},
		{
			name:    "filesystem path",
			input:   "/path/to/file",
			wantErr: errors.E("no hostname detected"),
		},
		{
			name:    "filesystem path with scheme",
			input:   "file:///path/to/file",
			wantErr: errors.E("no hostname detected"),
		},
		{
			name:      "github.com SSH URL",
			input:     "ssh://github.com/monalisa/octo-cat.git",
			wantHost:  "github.com",
			wantOwner: "monalisa",
			wantRepo:  "octo-cat",
		},
		{
			name:      "github.com HTTPS+SSH URL",
			input:     "https+ssh://github.com/monalisa/octo-cat.git",
			wantHost:  "github.com",
			wantOwner: "monalisa",
			wantRepo:  "octo-cat",
		},
		{
			name:      "github.com git URL",
			input:     "git://github.com/monalisa/octo-cat.git",
			wantHost:  "github.com",
			wantOwner: "monalisa",
			wantRepo:  "octo-cat",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.input)
			assert.NoError(t, err)
			host, owner, repo, err := git.RepoInfoFromURL(u)
			errtest.Assert(t, err, tt.wantErr)
			if err != nil {
				return
			}
			assert.NoError(t, err)
			assert.EqualStrings(t, tt.wantHost, host)
			assert.EqualStrings(t, tt.wantOwner, owner)
			assert.EqualStrings(t, tt.wantRepo, repo)
		})
	}
}
