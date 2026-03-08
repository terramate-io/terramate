// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package resolve_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// startTestRegistry starts a minimal OCI distribution-spec compliant HTTP server
// that serves a single repository with a single tag.
// It implements enough of the distribution spec for oras-go to pull:
//   - GET /v2/ (API version check)
//   - HEAD/GET /v2/<name>/manifests/<reference> (resolve + fetch manifest)
//   - HEAD/GET /v2/<name>/blobs/<digest> (fetch blobs)
func startTestRegistry(t *testing.T, repoName, tag string, manifestBytes []byte, blobs map[string][]byte) *httptest.Server {
	t.Helper()

	// Parse manifest to validate.
	var manifest ocispec.Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("failed to parse manifest: %v", err)
	}

	manifestDigest := sha256Digest(manifestBytes)

	mux := http.NewServeMux()

	// /v2/ - API version check.
	mux.HandleFunc("/v2/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if path == "/v2/" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// /v2/<name>/manifests/<reference>
		if strings.Contains(path, "/manifests/") {
			ref := path[strings.LastIndex(path, "/")+1:]
			if ref != tag && ref != manifestDigest {
				http.NotFound(w, r)
				return
			}

			w.Header().Set("Content-Type", ocispec.MediaTypeImageManifest)
			w.Header().Set("Docker-Content-Digest", manifestDigest)
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(manifestBytes)))

			if r.Method == http.MethodHead {
				return
			}
			w.Write(manifestBytes)
			return
		}

		// /v2/<name>/blobs/<digest>
		if strings.Contains(path, "/blobs/") {
			digest := path[strings.LastIndex(path, "/")+1:]
			data, ok := blobs[digest]
			if !ok {
				http.NotFound(w, r)
				return
			}

			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
			w.Header().Set("Docker-Content-Digest", digest)

			if r.Method == http.MethodHead {
				return
			}
			w.Write(data)
			return
		}

		http.NotFound(w, r)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

// sha256Digest returns the OCI-format sha256 digest of data.
func sha256Digest(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", h)
}
