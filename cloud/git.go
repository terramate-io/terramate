// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud

import (
	"fmt"
	"os"
	"strings"
)

// NormalizeGitURI normalizes the raw uri in a Terramate Cloud
// compatible form.
func NormalizeGitURI(raw string) string {
	// in the case the remote is a local bare repo, it can be an absolute or
	// a relative path, but relative paths can be ambiguous with remote URLs,
	// then an fs stat is needed here.
	_, err := os.Lstat(raw)
	if err == nil {
		// path exists, then likely a local path.
		return "local"
	}

	switch {
	case strings.HasPrefix(raw, "git@"):
		uri := raw[4:]
		parts := strings.Split(uri, ":")
		if len(parts) == 2 {
			host := parts[0]
			path := strings.TrimSuffix(parts[1], ".git")
			return fmt.Sprintf("%s/%s", host, path)
		}
		// unrecognized, then return it raw
	case strings.HasPrefix(raw, "https://"):
		uri := raw[8:]
		parts := strings.Split(uri, "/")
		if len(parts) > 1 {
			host := parts[0]
			path := strings.TrimSuffix(strings.Join(parts[1:], "/"), ".git")
			return fmt.Sprintf("%s/%s", host, path)
		}
	}
	return raw
}
