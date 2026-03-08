// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package resolve

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/errors"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

const (
	// OCIScheme is the URL scheme prefix for OCI sources.
	OCIScheme = "oci://"
)

// OCIReference holds parsed OCI reference fields.
type OCIReference struct {
	// Registry is the OCI registry host, e.g. "ghcr.io" or "123456.dkr.ecr.us-west-2.amazonaws.com".
	Registry string
	// Repository is the image repository path, e.g. "myorg/my-bundle".
	Repository string
	// Tag is the image tag, e.g. "v1.0.0". Defaults to "latest" if not specified.
	Tag string
	// Digest is the image digest, e.g. "sha256:abc...". Empty if tag-based.
	Digest string
	// Raw is the full reference string without the oci:// prefix (and without subdir).
	Raw string
}

// Reference returns the full reference string suitable for oras (registry/repo:tag or registry/repo@digest).
func (r *OCIReference) Reference() string {
	if r.Digest != "" {
		return fmt.Sprintf("%s/%s@%s", r.Registry, r.Repository, r.Digest)
	}
	return fmt.Sprintf("%s/%s:%s", r.Registry, r.Repository, r.Tag)
}

// IsOCISource returns true if the source string starts with "oci://".
func IsOCISource(src string) bool {
	return strings.HasPrefix(src, OCIScheme)
}

// ParseOCIReference parses an "oci://registry/repo:tag" string into an OCIReference.
// The input should be the source URL without any subdir suffix (already split by SourceDirSubdir).
func ParseOCIReference(src string) (*OCIReference, error) {
	raw := strings.TrimPrefix(src, OCIScheme)
	if raw == "" {
		return nil, errors.E("empty OCI reference")
	}

	ref := &OCIReference{Raw: raw}

	// Split registry from repo path.
	// The first path component is the registry host (may contain port).
	slashIdx := strings.Index(raw, "/")
	if slashIdx == -1 {
		return nil, errors.E("invalid OCI reference %q: missing repository path", src)
	}

	ref.Registry = raw[:slashIdx]
	remainder := raw[slashIdx+1:]

	if remainder == "" {
		return nil, errors.E("invalid OCI reference %q: empty repository", src)
	}

	// Check for digest (@sha256:...)
	if atIdx := strings.Index(remainder, "@"); atIdx != -1 {
		ref.Repository = remainder[:atIdx]
		ref.Digest = remainder[atIdx+1:]
		return ref, nil
	}

	// Check for tag (:tag)
	// Tags can only be in the last path component, so split from the last colon
	// that appears after the last slash.
	lastSlash := strings.LastIndex(remainder, "/")
	colonSearch := remainder
	if lastSlash != -1 {
		colonSearch = remainder[lastSlash:]
	}
	if colonIdx := strings.LastIndex(colonSearch, ":"); colonIdx != -1 {
		if lastSlash != -1 {
			colonIdx += lastSlash
		}
		ref.Repository = remainder[:colonIdx]
		ref.Tag = remainder[colonIdx+1:]
	} else {
		ref.Repository = remainder
		ref.Tag = "latest"
	}

	if ref.Repository == "" {
		return nil, errors.E("invalid OCI reference %q: empty repository", src)
	}

	return ref, nil
}

// fetchOCI pulls an OCI artifact and extracts its contents into cacheDir.
// It returns the manifest digest for use in content-addressable install paths.
func fetchOCI(ctx context.Context, ref *OCIReference, cacheDir string) (string, error) {
	logger := log.With().
		Str("action", "fetchOCI").
		Str("ref", ref.Reference()).
		Str("cache_dir", cacheDir).
		Logger()

	logger.Debug().Msg("fetching OCI artifact")

	repo, err := remote.NewRepository(ref.Registry + "/" + ref.Repository)
	if err != nil {
		return "", errors.E(err, "creating OCI repository client for %s", ref.Reference())
	}

	// Allow plain HTTP for localhost registries (development/testing).
	host := ref.Registry
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}
	if host == "localhost" || host == "127.0.0.1" {
		repo.PlainHTTP = true
	}

	// Set up authentication using Docker credential store.
	// https://oras.land/docs/how_to_guides/authentication
	credStore, err := newOCICredentialStore()
	if err != nil {
		logger.Debug().Err(err).Msg("failed to create credential store, proceeding without auth")
	} else {
		repo.Client = &auth.Client{
			Credential: credentials.Credential(credStore),
		}
	}

	// Resolve the tag/digest to a descriptor to get the manifest.
	tagOrDigest := ref.Tag
	if ref.Digest != "" {
		tagOrDigest = ref.Digest
	}

	desc, err := repo.Resolve(ctx, tagOrDigest)
	if err != nil {
		return "", errors.E(err, "resolving OCI reference %s", ref.Reference())
	}

	manifestDigest := desc.Digest.String()
	logger.Debug().Str("digest", manifestDigest).Msg("resolved OCI manifest")

	// Fetch the manifest to find layers.
	rc, err := repo.Fetch(ctx, desc)
	if err != nil {
		return "", errors.E(err, "fetching OCI manifest for %s", ref.Reference())
	}

	manifestBytes, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return "", errors.E(err, "reading OCI manifest")
	}

	// Parse manifest to find layers.
	manifest, err := parseOCIManifest(manifestBytes)
	if err != nil {
		return "", errors.E(err, "parsing OCI manifest")
	}

	// Extract tar.gz layers into cacheDir.
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", errors.E(err, "creating cache directory")
	}

	extracted := false
	for _, layer := range manifest.Layers {
		if IsTarGzipLayer(layer.MediaType) {
			logger.Debug().
				Str("media_type", layer.MediaType).
				Int64("size", layer.Size).
				Msg("extracting layer")

			layerRC, err := repo.Blobs().Fetch(ctx, layer)
			if err != nil {
				return "", errors.E(err, "fetching OCI layer %s", layer.Digest)
			}

			if err := ExtractTarGz(layerRC, cacheDir); err != nil {
				layerRC.Close()
				return "", errors.E(err, "extracting OCI layer %s", layer.Digest)
			}
			layerRC.Close()
			extracted = true
		}
	}

	if !extracted {
		return "", errors.E("OCI artifact %s contains no extractable layers (expected tar+gzip)", ref.Reference())
	}

	return manifestDigest, nil
}

// newOCICredentialStore creates a credential store from the Docker config.
func newOCICredentialStore() (credentials.Store, error) {
	store, err := credentials.NewStoreFromDocker(credentials.StoreOptions{
		AllowPlaintextPut: false,
	})
	if err != nil {
		return nil, errors.E(err, "creating Docker credential store")
	}
	return store, nil
}

// IsTarGzipLayer returns true if the media type indicates a tar+gzip layer.
func IsTarGzipLayer(mediaType string) bool {
	switch mediaType {
	case "application/vnd.oci.image.layer.v1.tar+gzip",
		"application/vnd.docker.image.rootfs.diff.tar.gzip",
		"application/vnd.terramate.bundle.content.v1.tar+gzip",
		"application/vnd.terramate.component.content.v1.tar+gzip",
		"application/vnd.cncf.helm.chart.content.v1.tar+gzip",
		"application/tar+gzip",
		"application/x-tar+gzip":
		return true
	}
	// Also match any media type ending in .tar+gzip or tar.gz.
	return strings.HasSuffix(mediaType, ".tar+gzip") || strings.HasSuffix(mediaType, ".tar.gzip")
}

// parseOCIManifest parses an OCI manifest from JSON bytes.
func parseOCIManifest(data []byte) (*v1.Manifest, error) {
	var manifest v1.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, errors.E(err, "unmarshaling OCI manifest")
	}
	return &manifest, nil
}

// ExtractTarGz extracts a tar.gz stream into the given directory.
func ExtractTarGz(r io.Reader, dir string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return errors.E(err, "creating gzip reader")
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.E(err, "reading tar entry")
		}

		// Sanitize path to prevent directory traversal.
		// https://owasp.org/www-community/attacks/Path_Traversal
		cleanName := filepath.Clean(hdr.Name)
		if strings.HasPrefix(cleanName, "..") {
			return errors.E("tar entry %q attempts directory traversal", hdr.Name)
		}

		target := filepath.Join(dir, cleanName)

		// Ensure the target is within the destination directory.
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dir)) {
			return errors.E("tar entry %q escapes destination directory", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return errors.E(err, "creating directory %s", target)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return errors.E(err, "creating parent directory for %s", target)
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0o755)
			if err != nil {
				return errors.E(err, "creating file %s", target)
			}
			// Limit copy size to prevent decompression bombs (256MB per file).
			if _, err := io.Copy(f, io.LimitReader(tr, 256<<20)); err != nil {
				f.Close()
				return errors.E(err, "writing file %s", target)
			}
			f.Close()
		case tar.TypeSymlink:
			// Validate symlink target doesn't escape.
			linkTarget := filepath.Clean(filepath.Join(filepath.Dir(target), hdr.Linkname))
			if !strings.HasPrefix(linkTarget, filepath.Clean(dir)) {
				return errors.E("symlink %q -> %q escapes destination directory", hdr.Name, hdr.Linkname)
			}
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return errors.E(err, "creating symlink %s", target)
			}
		}
	}

	return nil
}
