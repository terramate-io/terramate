// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package git

import (
	"net/url"
	"os"
	"strings"

	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/strconv"
)

// ErrInvalidGitURL indicates is an error kind indicating the git URL is not valid.
const ErrInvalidGitURL errors.Kind = "invalid git remote URL"

// Repository aggregates information about a repository.
type Repository struct {
	RawURL string // original remote URL (eg.: git@github.com:terramate-io/terramate.git)
	Host   string // Host of the remote URL.
	Repo   string // Normalized repository name (eg.: github.com/terramate-io/terramate)
	Owner  string // Owner of the repository (eg.: terramate-io)
	Name   string // Name of the repository (including groups, if any) (eg.: terramate)
}

// NormalizeGitURI parses the raw git remote URL and returns a normalized
// repository type.
func NormalizeGitURI(raw string) (Repository, error) {
	r := Repository{
		RawURL: raw,
	}
	// in the case the remote is a local bare repo, it can be an absolute or
	// a relative path, but relative paths can be ambiguous with remote URLs,
	// then an fs stat is needed here.
	_, err := os.Lstat(raw)
	if err == nil {
		// path exists, then likely a local path.
		r.Host = "local"
		// NOTE: other fields are not set for "local" repositories.
		return r, nil
	}

	if !IsURL(raw) {
		return Repository{}, errors.E(ErrInvalidGitURL, "unsupported git URL: %s", raw)
	}

	u, err := ParseURL(raw)
	if err != nil {
		return Repository{}, errors.E(ErrInvalidGitURL, err)
	}

	host, owner, name, err := RepoInfoFromURL(u)
	if err != nil {
		return Repository{}, errors.E(ErrInvalidGitURL, err)
	}
	r.Host = host
	r.Owner = owner
	r.Name = name
	r.Repo = r.Host
	if r.Owner != "" {
		r.Repo += "/" + r.Owner
	}
	if r.Name != "" {
		r.Repo += "/" + r.Name
	}
	return r, nil
}

// IsURL tells if the u URL is a supported git remote URL.
func IsURL(u string) bool {
	if strings.HasPrefix(u, "git@") || isSupportedProtocol(u) {
		return true
	}
	index := strings.Index(u, ":")
	// any other <schema>:// is not supported
	return index > 0 && !strings.HasPrefix(u[:index], "://")
}

func isSupportedProtocol(u string) bool {
	return strings.HasPrefix(u, "ssh:") ||
		strings.HasPrefix(u, "git+ssh:") ||
		strings.HasPrefix(u, "git:") ||
		strings.HasPrefix(u, "http:") ||
		strings.HasPrefix(u, "git+https:") ||
		strings.HasPrefix(u, "https:")
}

func isPossibleProtocol(u string) bool {
	return isSupportedProtocol(u) ||
		strings.HasPrefix(u, "ftp:") ||
		strings.HasPrefix(u, "ftps:") ||
		strings.HasPrefix(u, "file:")
}

// ParseURL normalizes git remote urls.
func ParseURL(rawURL string) (u *url.URL, err error) {
	if !isPossibleProtocol(rawURL) &&
		strings.ContainsRune(rawURL, ':') &&
		// Not a Windows path.
		!strings.ContainsRune(rawURL, '\\') {
		// Support scp-like syntax for ssh protocol.
		// We convert SCP syntax into ssh://<uri>
		// Examples below:
		// git@github.com:some/path.git -> ssh://github.com/some/path.git
		// git@github.com:2222/some/path.git -> ssh://github.com:2222/some/path.git
		index := strings.Index(rawURL, ":")
		if index > 0 {
			next := strings.Index(rawURL[index+1:], "/")
			if next > 0 {
				// check if port is present
				_, err := strconv.Atoi64(rawURL[index+1 : index+1+next])
				if err == nil {
					index = -1
				}
			}
		}
		strRunes := []rune(rawURL)
		if index > 0 {
			strRunes[index] = '/'
		}
		rawURL = "ssh://" + string(strRunes)
	}
	u, err = url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "git+ssh" {
		u.Scheme = "ssh"
	}
	if u.Scheme == "git+https" {
		u.Scheme = "https"
	}
	if u.Scheme != "ssh" {
		return
	}
	if strings.HasPrefix(u.Path, "//") {
		u.Path = strings.TrimPrefix(u.Path, "/")
	}
	return u, nil
}

// RepoInfoFromURL returns the host, owner and repo name from a given URL.
func RepoInfoFromURL(u *url.URL) (host string, owner string, name string, err error) {
	if u.Hostname() == "" {
		return "", "", "", errors.E("no hostname detected")
	}
	parts := strings.SplitN(strings.Trim(u.Path, "/"), "/", 2)
	if len(parts) == 2 {
		owner = parts[0]
		name = parts[1]
	} else {
		name = parts[0]
	}
	name = strings.TrimSuffix(name, ".git")
	return normalizeHostname(u), owner, name, nil
}

func normalizeHostname(u *url.URL) string {
	host := u.Hostname()
	if p := u.Port(); p != "" && p != "80" && p != "443" {
		host += ":" + p
	}
	return strings.ToLower(strings.TrimPrefix(host, "www."))
}
