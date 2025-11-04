// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tg

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/terramate-io/terramate/git"
)

// FileExistsCache creates a memory-efficient caching data structure to speed up
// file exist checks for a given directory tree.
type FileExistsCache struct {
	data map[string]struct{}
}

// IsHCLFile checks if a file has a regular .hcl extension (not .tm.hcl)
func IsHCLFile(fn string) bool {
	return !strings.HasSuffix(fn, ".tm.hcl") && strings.HasSuffix(fn, ".hcl")
}

// NewFileExistsCache creates a new cache.
func NewFileExistsCache(root string, fnPred func(fn string) bool) (*FileExistsCache, error) {
	cache := &FileExistsCache{
		data: map[string]struct{}{},
	}

	g, err := git.WithConfig(git.Config{
		WorkingDir:     root,
		Env:            os.Environ(),
		AllowPorcelain: true,
	})
	if err != nil {
		return nil, err
	}

	out, err := g.Exec("ls-files")
	if err != nil {
		return nil, err
	}

	root = filepath.ToSlash(root)

	for path := range strings.SplitSeq(out, "\n") {
		fullPath := fmt.Sprintf("%s/%s", root, path)
		if fnPred(path) {
			cache.data[fullPath] = struct{}{}
		}
	}

	return cache, nil
}

// FileExists checks if a file exists. The second return value indicates if Stat call was used to check if the file exists (true),
// or if the cache was used to skip it (false).
func (c *FileExistsCache) FileExists(path string) bool {
	path = filepath.ToSlash(path)
	_, found := c.data[path]
	return found
}
