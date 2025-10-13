// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tmls

import (
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestDocumentSyncLifecycle(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		`f:globals.tm:globals {
  region = "us-east-1"
}`,
	})

	srv := newTestServer(t, s.RootDir())
	fname := filepath.Join(s.RootDir(), "globals.tm")

	// Initially, document is not cached - will read from disk
	cached, err := srv.getDocumentContent(fname)
	assert.NoError(t, err)
	assert.IsTrue(t, cached != nil, "should read from disk when not cached")

	// Simulate document open - cache the content
	originalContent := []byte(`globals {
  region = "us-east-1"
}`)
	srv.setDocumentContent(fname, originalContent)

	// Content should now be cached
	cached, err = srv.getDocumentContent(fname)
	assert.NoError(t, err)
	assert.IsTrue(t, cached != nil, "document should be cached after open")
	assert.EqualStrings(t, string(originalContent), string(cached))

	// Simulate document change - update cache
	newContent := []byte(`globals {
  region = "us-west-2"  # changed
}`)
	srv.setDocumentContent(fname, newContent)

	// Cache should be updated
	cached, err = srv.getDocumentContent(fname)
	assert.NoError(t, err)
	assert.IsTrue(t, cached != nil, "document should still be cached after change")
	assert.EqualStrings(t, string(newContent), string(cached))

	// Simulate document close - clear cache
	srv.deleteDocumentContent(fname)

	// Should fall back to reading from disk
	cached, err = srv.getDocumentContent(fname)
	assert.NoError(t, err)
	assert.IsTrue(t, cached != nil, "should fall back to disk after cache cleared")
}

func TestDocumentContentFallback(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		`f:globals.tm:globals {
  value = "test"
}`,
	})

	srv := newTestServer(t, s.RootDir())
	fname := filepath.Join(s.RootDir(), "globals.tm")

	// When document is not cached, getDocumentContent should read from disk
	content, err := srv.getDocumentContent(fname)
	assert.NoError(t, err)
	assert.IsTrue(t, content != nil, "should read content from disk")
	assert.IsTrue(t, len(content) > 0, "content should not be empty")
}

func TestSetAndDeleteDocumentContent(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		`f:globals.tm:globals {
  region = "us-east-1"
}`,
	})

	srv := newTestServer(t, s.RootDir())
	fname := filepath.Join(s.RootDir(), "globals.tm")

	// Set document content
	originalContent := []byte(`globals {
  region = "us-east-1"
}`)
	srv.setDocumentContent(fname, originalContent)

	// Verify content is cached
	cached, err := srv.getDocumentContent(fname)
	assert.NoError(t, err)
	assert.EqualStrings(t, string(originalContent), string(cached))

	// Update document content
	changedContent := []byte(`globals {
  region = "us-west-2"
}`)
	srv.setDocumentContent(fname, changedContent)

	// Verify cache is updated
	cached, err = srv.getDocumentContent(fname)
	assert.NoError(t, err)
	assert.EqualStrings(t, string(changedContent), string(cached))

	// Delete document content
	srv.deleteDocumentContent(fname)

	// After delete, should fall back to disk
	cached, err = srv.getDocumentContent(fname)
	assert.NoError(t, err)
	assert.IsTrue(t, cached != nil, "should fall back to disk after delete")
}
