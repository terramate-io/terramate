// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package run

import "sync"

type lookpathEntry struct {
	once sync.Once
	path string
	err  error
}

var (
	lookpathCacheMu sync.Mutex
	lookpathCache   = map[string]*lookpathEntry{}
)

func getLookpathEntry(key string) *lookpathEntry {
	lookpathCacheMu.Lock()
	defer lookpathCacheMu.Unlock()
	entry, ok := lookpathCache[key]
	if !ok {
		entry = &lookpathEntry{}
		lookpathCache[key] = entry
	}
	return entry
}

// LookPath resolves the path to the given executable name using the provided
// environment. The resolution is performed at most once per process for a given
// file identifier. Subsequent calls for the same file return the cached result
// even if PATH changes.
func LookPath(file string, environ []string) (string, error) {
	entry := getLookpathEntry(file)
	entry.once.Do(func() {
		entry.path, entry.err = lookPathImpl(file, environ)
	})
	// Do not cache failures: allow re-attempts with potentially different PATH.
	if entry.err != nil || entry.path == "" {
		lookpathCacheMu.Lock()
		delete(lookpathCache, file)
		lookpathCacheMu.Unlock()
	}
	return entry.path, entry.err
}

// ResetLookPathCache clears the shared LookPath cache. Intended for tests.
func ResetLookPathCache() {
	lookpathCacheMu.Lock()
	defer lookpathCacheMu.Unlock()
	lookpathCache = map[string]*lookpathEntry{}
}
