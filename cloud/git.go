// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud

import (
	"fmt"
	"os"

	"github.com/cli/go-gh/v2/pkg/repository"
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

	repo, err := repository.Parse(raw)
	if err != nil {
		return raw
	}

	return fmt.Sprintf("%s/%s/%s", repo.Host, repo.Owner, repo.Name)
}
