// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package resolve_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/terramate-io/terramate/generate/resolve"

	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/memory"
)

// createTarGz creates a tar.gz archive from a map of filename -> content.
func createTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}

	tw.Close()
	gw.Close()
	return buf.Bytes()
}

// pushTestArtifact creates an OCI artifact in memory and returns the manifest bytes and blob map.
func pushTestArtifact(t *testing.T, tag string, files map[string]string) ([]byte, map[string][]byte) {
	t.Helper()

	tarGzContent := createTarGz(t, files)
	ctx := context.Background()
	memStore := memory.New()

	layerDesc, err := oras.PushBytes(ctx, memStore, "application/vnd.oci.image.layer.v1.tar+gzip", tarGzContent)
	if err != nil {
		t.Fatal(err)
	}

	configDesc, err := oras.PushBytes(ctx, memStore, "application/vnd.oci.image.config.v1+json", []byte("{}"))
	if err != nil {
		t.Fatal(err)
	}

	manifest := ocispec.Manifest{
		MediaType: ocispec.MediaTypeImageManifest,
		Config:    configDesc,
		Layers:    []ocispec.Descriptor{layerDesc},
	}
	manifest.SchemaVersion = 2

	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}

	blobs := map[string][]byte{
		layerDesc.Digest.String():  tarGzContent,
		configDesc.Digest.String(): []byte("{}"),
	}

	return manifestBytes, blobs
}

// TestExtractTarGz tests the tar.gz extraction logic directly.
func TestExtractTarGz(t *testing.T) {
	t.Parallel()

	files := map[string]string{
		"bundle.tm.hcl":          `bundle { name = "test" }`,
		"components/net/main.tf": `resource "aws_vpc" "main" {}`,
		"components/net/vars.tf": `variable "cidr" {}`,
		"README.md":              "# Test Bundle",
	}

	tarGzData := createTarGz(t, files)

	dir := t.TempDir()
	if err := resolve.ExtractTarGz(bytes.NewReader(tarGzData), dir); err != nil {
		t.Fatal(err)
	}

	for name, wantContent := range files {
		path := filepath.Join(dir, name)
		got, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("file %s not found: %v", name, err)
			continue
		}
		assert.EqualStrings(t, wantContent, string(got))
	}
}

// TestExtractTarGzDirectoryTraversal verifies path traversal attacks are rejected.
func TestExtractTarGzDirectoryTraversal(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	hdr := &tar.Header{
		Name: "../../../etc/passwd",
		Mode: 0o644,
		Size: 5,
	}
	tw.WriteHeader(hdr)
	tw.Write([]byte("pwned"))
	tw.Close()
	gw.Close()

	dir := t.TempDir()
	err := resolve.ExtractTarGz(bytes.NewReader(buf.Bytes()), dir)
	if err == nil {
		t.Fatal("expected error for directory traversal, got nil")
	}
	if !strings.Contains(err.Error(), "directory traversal") {
		t.Fatalf("expected directory traversal error, got: %v", err)
	}
}

// TestExtractTarGzNestedDirs verifies extraction of nested directory structures.
func TestExtractTarGzNestedDirs(t *testing.T) {
	t.Parallel()

	files := map[string]string{
		"a/b/c/deep.hcl": `deep = true`,
		"top.hcl":         `top = true`,
	}

	tarGzData := createTarGz(t, files)
	dir := t.TempDir()

	if err := resolve.ExtractTarGz(bytes.NewReader(tarGzData), dir); err != nil {
		t.Fatal(err)
	}

	for name, wantContent := range files {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Errorf("missing %s: %v", name, err)
			continue
		}
		assert.EqualStrings(t, wantContent, string(got))
	}
}

// TestIsTarGzipLayer verifies media type detection.
func TestIsTarGzipLayer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mediaType string
		want      bool
	}{
		{"application/vnd.oci.image.layer.v1.tar+gzip", true},
		{"application/vnd.docker.image.rootfs.diff.tar.gzip", true},
		{"application/vnd.terramate.bundle.content.v1.tar+gzip", true},
		{"application/vnd.terramate.component.content.v1.tar+gzip", true},
		{"application/vnd.cncf.helm.chart.content.v1.tar+gzip", true},
		{"application/tar+gzip", true},
		{"application/x-tar+gzip", true},
		{"application/vnd.custom.tar+gzip", true},
		{"application/json", false},
		{"application/vnd.oci.image.config.v1+json", false},
		{"text/plain", false},
		{"", false},
	}

	for _, tc := range tests {
		t.Run(tc.mediaType, func(t *testing.T) {
			got := resolve.IsTarGzipLayer(tc.mediaType)
			if got != tc.want {
				t.Errorf("IsTarGzipLayer(%q) = %v, want %v", tc.mediaType, got, tc.want)
			}
		})
	}
}

// TestResolveOCIFullFlow tests the full OCI resolution flow against an in-process registry.
func TestResolveOCIFullFlow(t *testing.T) {
	t.Parallel()

	bundleFiles := map[string]string{
		"bundle.tm.hcl": `bundle {
  name = "test-bundle"
}

component "network" {
  source = "./components/network"
}
`,
		"components/network/main.tf": `resource "aws_vpc" "main" {
  cidr_block = var.cidr
}
`,
		"components/network/variables.tf": `variable "cidr" {
  type = string
}
`,
	}

	manifestBytes, blobs := pushTestArtifact(t, "v1.0.0", bundleFiles)

	server := startTestRegistry(t, "test-bundle", "v1.0.0", manifestBytes, blobs)
	defer server.Close()

	registryURL, _ := url.Parse(server.URL)
	registryHost := registryURL.Host

	cacheDir := t.TempDir()
	rootDir := t.TempDir()
	resolver := resolve.NewResolverForTest(cacheDir)

	src := fmt.Sprintf("oci://%s/test-bundle:v1.0.0", registryHost)
	result, err := resolver.Resolve(rootDir, src, resolve.Bundle, true)
	if err != nil {
		t.Fatalf("Resolve(%q) failed: %v", src, err)
	}

	installDir := filepath.Join(rootDir, result.String())
	for name, wantContent := range bundleFiles {
		got, err := os.ReadFile(filepath.Join(installDir, name))
		if err != nil {
			t.Errorf("expected file %s not found in install dir: %v", name, err)
			continue
		}
		assert.EqualStrings(t, wantContent, string(got))
	}
}

// TestResolveOCICacheHit verifies that a second resolve uses the cache.
func TestResolveOCICacheHit(t *testing.T) {
	t.Parallel()

	bundleFiles := map[string]string{
		"main.tf": `resource "null_resource" "test" {}`,
	}

	manifestBytes, blobs := pushTestArtifact(t, "v1.0.0", bundleFiles)

	server := startTestRegistry(t, "cache-test", "v1.0.0", manifestBytes, blobs)
	defer server.Close()

	registryURL, _ := url.Parse(server.URL)
	registryHost := registryURL.Host

	cacheDir := t.TempDir()
	rootDir := t.TempDir()
	resolver := resolve.NewResolverForTest(cacheDir)

	src := fmt.Sprintf("oci://%s/cache-test:v1.0.0", registryHost)

	// First resolve - fetches from registry.
	result1, err := resolver.Resolve(rootDir, src, resolve.Bundle, true)
	if err != nil {
		t.Fatalf("first Resolve failed: %v", err)
	}

	// Second resolve with allowFetch=false - should succeed from cache.
	result2, err := resolver.Resolve(rootDir, src, resolve.Bundle, false)
	if err != nil {
		t.Fatalf("second Resolve (cached) failed: %v", err)
	}

	assert.EqualStrings(t, result1.String(), result2.String())
}

// TestResolveOCINotAllowedFetch verifies that allowFetch=false fails on cache miss.
func TestResolveOCINotAllowedFetch(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()
	rootDir := t.TempDir()
	resolver := resolve.NewResolverForTest(cacheDir)

	src := "oci://ghcr.io/nonexistent/bundle:v1.0.0"
	_, err := resolver.Resolve(rootDir, src, resolve.Bundle, false)
	if err == nil {
		t.Fatal("expected error for cache miss with allowFetch=false")
	}
}

// TestResolveOCIWithSubdir tests OCI resolution with a subdir path.
func TestResolveOCIWithSubdir(t *testing.T) {
	t.Parallel()

	bundleFiles := map[string]string{
		"bundles/network/bundle.tm.hcl": `bundle { name = "network" }`,
		"bundles/compute/bundle.tm.hcl": `bundle { name = "compute" }`,
		"components/vpc/main.tf":        `resource "aws_vpc" "main" {}`,
	}

	manifestBytes, blobs := pushTestArtifact(t, "v2.0.0", bundleFiles)

	server := startTestRegistry(t, "multi-bundle", "v2.0.0", manifestBytes, blobs)
	defer server.Close()

	registryURL, _ := url.Parse(server.URL)
	registryHost := registryURL.Host

	cacheDir := t.TempDir()
	rootDir := t.TempDir()
	resolver := resolve.NewResolverForTest(cacheDir)

	src := fmt.Sprintf("oci://%s/multi-bundle:v2.0.0//bundles/network", registryHost)
	result, err := resolver.Resolve(rootDir, src, resolve.Bundle, true)
	if err != nil {
		t.Fatalf("Resolve with subdir failed: %v", err)
	}

	installDir := filepath.Join(rootDir, result.String())
	got, err := os.ReadFile(filepath.Join(installDir, "bundle.tm.hcl"))
	if err != nil {
		t.Fatalf("expected bundle.tm.hcl in subdir install: %v", err)
	}
	assert.EqualStrings(t, `bundle { name = "network" }`, string(got))
}
