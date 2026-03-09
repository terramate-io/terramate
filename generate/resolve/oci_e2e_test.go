// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package resolve_test

import (
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
	"oras.land/oras-go/v2/registry/remote"
)

// TestOrasPushThenResolve simulates the real-world workflow:
//  1. User creates a bundle directory
//  2. User runs `oras push` (simulated via oras-go library) to push the tar.gz to a registry
//  3. Consumer uses `oci://` source to resolve and install the bundle
//
// This validates the exact workflow described in the issue using the same oras-go
// library that the `oras` CLI uses internally.
func TestOrasPushThenResolve(t *testing.T) {
	t.Parallel()

	// Step 1: Simulate "terramate package create" output — a bundle directory.
	bundleDir := t.TempDir()
	bundleFiles := map[string]string{
		"bundle.tm.hcl": `bundle {
  name        = "data-plane-aws"
  description = "AWS data plane infrastructure"
}

component "network" {
  source = "./components/network"
  inputs = {
    cidr = input.cidr
  }
}

component "compute" {
  source = "./components/compute"
  inputs = {
    instance_type = input.instance_type
  }
}
`,
		"components/network/main.tf": `resource "aws_vpc" "main" {
  cidr_block = var.cidr

  tags = {
    Name = "data-plane"
  }
}

resource "aws_subnet" "private" {
  vpc_id     = aws_vpc.main.id
  cidr_block = cidrsubnet(var.cidr, 8, 1)
}
`,
		"components/network/variables.tf": `variable "cidr" {
  type        = string
  description = "VPC CIDR block"
}
`,
		"components/compute/main.tf": `resource "aws_instance" "worker" {
  ami           = data.aws_ami.ubuntu.id
  instance_type = var.instance_type
  subnet_id     = var.subnet_id
}
`,
		"components/compute/variables.tf": `variable "instance_type" {
  type    = string
  default = "t3.medium"
}

variable "subnet_id" {
  type = string
}
`,
	}

	for name, content := range bundleFiles {
		path := filepath.Join(bundleDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Step 2: Simulate `tar czf bundle.tar.gz -C ./dist .`
	tarGzData := createTarGz(t, bundleFiles)

	// Step 3: Simulate `oras push registry/bundles/data-plane-aws:v2.0.0 bundle.tar.gz`
	// This uses the same oras-go library that the oras CLI uses.
	ctx := context.Background()
	memStore := memory.New()

	layerDesc, err := oras.PushBytes(ctx, memStore, "application/vnd.oci.image.layer.v1.tar+gzip", tarGzData)
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
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}

	// Start our test registry and push the artifact.
	blobs := map[string][]byte{
		layerDesc.Digest.String():  tarGzData,
		configDesc.Digest.String(): []byte("{}"),
	}
	server := startTestRegistry(t, "bundles/data-plane-aws", "v2.0.0", manifestBytes, blobs)

	registryURL, _ := url.Parse(server.URL)
	registryHost := registryURL.Host

	// Verify the artifact is accessible via oras-go (same as `oras pull` would do).
	repo, err := remote.NewRepository(registryHost + "/bundles/data-plane-aws")
	if err != nil {
		t.Fatal(err)
	}
	repo.PlainHTTP = true

	desc, err := repo.Resolve(ctx, "v2.0.0")
	if err != nil {
		t.Fatalf("oras resolve failed (simulates `oras manifest fetch`): %v", err)
	}
	t.Logf("Pushed artifact digest: %s", desc.Digest)

	// Step 4: Consumer resolves the OCI source — this is what happens when
	// a bundle instance references `source: oci://registry/bundles/data-plane-aws:v2.0.0`
	cacheDir := t.TempDir()
	rootDir := t.TempDir()
	resolver := resolve.NewResolverForTest(cacheDir)

	src := fmt.Sprintf("oci://%s/bundles/data-plane-aws:v2.0.0", registryHost)
	result, err := resolver.Resolve(rootDir, src, resolve.Bundle, true)
	if err != nil {
		t.Fatalf("Resolve(%q) failed: %v", src, err)
	}

	installDir := filepath.Join(rootDir, result.String())
	t.Logf("Bundle installed to: %s", installDir)

	// Step 5: Verify all bundle files were extracted correctly.
	for name, wantContent := range bundleFiles {
		got, err := os.ReadFile(filepath.Join(installDir, name))
		if err != nil {
			t.Errorf("expected file %s not found in install dir: %v", name, err)
			continue
		}
		assert.EqualStrings(t, wantContent, string(got))
	}

	// Verify directory structure is correct.
	for _, dir := range []string{"components/network", "components/compute"} {
		info, err := os.Stat(filepath.Join(installDir, dir))
		if err != nil {
			t.Errorf("expected directory %s not found: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %s to be a directory", dir)
		}
	}

	t.Log("SUCCESS: Full oras push → terramate resolve workflow verified")
}

// TestOrasPushComponentThenResolve tests pushing and resolving a single component.
func TestOrasPushComponentThenResolve(t *testing.T) {
	t.Parallel()

	componentFiles := map[string]string{
		"main.tf": `resource "aws_vpc" "main" {
  cidr_block = var.cidr
}
`,
		"variables.tf": `variable "cidr" {
  type = string
}
`,
		"outputs.tf": `output "vpc_id" {
  value = aws_vpc.main.id
}
`,
	}

	tarGzData := createTarGz(t, componentFiles)
	ctx := context.Background()
	memStore := memory.New()

	layerDesc, err := oras.PushBytes(ctx, memStore, "application/vnd.oci.image.layer.v1.tar+gzip", tarGzData)
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
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}

	blobs := map[string][]byte{
		layerDesc.Digest.String():  tarGzData,
		configDesc.Digest.String(): []byte("{}"),
	}
	server := startTestRegistry(t, "components/network-aws", "v1.0.0", manifestBytes, blobs)
	registryURL, _ := url.Parse(server.URL)

	cacheDir := t.TempDir()
	rootDir := t.TempDir()
	resolver := resolve.NewResolverForTest(cacheDir)

	// Resolve as a Component kind.
	src := fmt.Sprintf("oci://%s/components/network-aws:v1.0.0", registryURL.Host)
	result, err := resolver.Resolve(rootDir, src, resolve.Component, true)
	if err != nil {
		t.Fatalf("Resolve component failed: %v", err)
	}

	installDir := filepath.Join(rootDir, result.String())
	for name, wantContent := range componentFiles {
		got, err := os.ReadFile(filepath.Join(installDir, name))
		if err != nil {
			t.Errorf("missing %s: %v", name, err)
			continue
		}
		assert.EqualStrings(t, wantContent, string(got))
	}

	t.Log("SUCCESS: Component push → resolve workflow verified")
}

// TestOrasPushBundleWithRelativeComponents tests that a bundle with relative
// component references works correctly when pulled from OCI.
// This validates the CombineSources logic with OCI parent sources.
func TestOrasPushBundleWithRelativeComponents(t *testing.T) {
	t.Parallel()

	// Bundle with relative component sources — both the bundle and components
	// are in the same OCI artifact under different subdirectories.
	files := map[string]string{
		"bundle/bundle.tm.hcl": `bundle {
  name = "infra"
}

component "network" {
  source = "../components/network"
}
`,
		"components/network/main.tf": `resource "aws_vpc" "main" {
  cidr_block = "10.0.0.0/16"
}
`,
	}

	tarGzData := createTarGz(t, files)
	ctx := context.Background()
	memStore := memory.New()

	layerDesc, err := oras.PushBytes(ctx, memStore, "application/vnd.oci.image.layer.v1.tar+gzip", tarGzData)
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
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}

	blobs := map[string][]byte{
		layerDesc.Digest.String():  tarGzData,
		configDesc.Digest.String(): []byte("{}"),
	}
	server := startTestRegistry(t, "infra-mono", "v1.0.0", manifestBytes, blobs)
	registryURL, _ := url.Parse(server.URL)
	registryHost := registryURL.Host

	cacheDir := t.TempDir()
	rootDir := t.TempDir()
	resolver := resolve.NewResolverForTest(cacheDir)

	// Resolve the bundle via its subdir.
	bundleSrc := fmt.Sprintf("oci://%s/infra-mono:v1.0.0//bundle", registryHost)
	bundleResult, err := resolver.Resolve(rootDir, bundleSrc, resolve.Bundle, true)
	if err != nil {
		t.Fatalf("Resolve bundle failed: %v", err)
	}

	bundleDir := filepath.Join(rootDir, bundleResult.String())
	got, err := os.ReadFile(filepath.Join(bundleDir, "bundle.tm.hcl"))
	if err != nil {
		t.Fatalf("bundle.tm.hcl not found: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("bundle.tm.hcl is empty")
	}

	// Now resolve the component using the bundle as parent source.
	// This is what happens when the engine processes `source = "../components/network"`
	// with the bundle's OCI source as parent.
	componentSrc := "../components/network"
	combined := resolve.CombineSources(componentSrc, bundleSrc)
	expectedCombined := fmt.Sprintf("oci://%s/infra-mono:v1.0.0//components/network", registryHost)
	assert.EqualStrings(t, expectedCombined, combined)

	componentResult, err := resolver.Resolve(rootDir, combined, resolve.Component, true)
	if err != nil {
		t.Fatalf("Resolve relative component failed: %v", err)
	}

	componentDir := filepath.Join(rootDir, componentResult.String())
	gotTF, err := os.ReadFile(filepath.Join(componentDir, "main.tf"))
	if err != nil {
		t.Fatalf("component main.tf not found: %v", err)
	}
	if !strings.Contains(string(gotTF), "aws_vpc") {
		t.Fatalf("component main.tf doesn't contain expected resource, got: %s", gotTF)
	}

	t.Log("SUCCESS: Bundle with relative component resolution via OCI verified")
}

