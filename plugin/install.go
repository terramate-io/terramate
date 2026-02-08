// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/hashicorp/go-version"
)

// InstallOptions configures plugin installation.
type InstallOptions struct {
	UserTerramateDir string
	RegistryURL      string
	Version          string
	Source           string
}

// ParseNameVersion splits name@version into name and version.
func ParseNameVersion(value string) (string, string) {
	if value == "" {
		return "", ""
	}
	parts := strings.Split(value, "@")
	if len(parts) == 1 {
		return value, ""
	}
	name := strings.Join(parts[:len(parts)-1], "@")
	return name, parts[len(parts)-1]
}

// Install downloads and installs a plugin from the registry.
func Install(ctx context.Context, name string, opts InstallOptions) (Manifest, error) {
	var m Manifest
	if opts.Source != "" {
		return InstallFromLocal(ctx, name, opts)
	}
	registryURL := opts.RegistryURL
	client := NewRegistryClient(registryURL)
	index, err := client.FetchIndex(ctx, name)
	if err != nil {
		return m, err
	}
	versionEntry, err := selectVersion(index.Versions, opts.Version)
	if err != nil {
		return m, err
	}
	asset, ok := versionEntry.SelectAsset()
	if !ok {
		return m, fmt.Errorf("no plugin binaries for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	pluginDir := PluginDir(opts.UserTerramateDir, name)
	if err := os.MkdirAll(pluginDir, 0o700); err != nil {
		return m, err
	}
	binaries := map[BinaryKind]Binary{}
	for kind, bin := range asset.Binaries {
		if bin.URL == "" || bin.Name == "" {
			continue
		}
		signaturePath := ""
		publicKeyPath := ""
		if bin.SignatureURL != "" && bin.PublicKeyURL != "" {
			signaturePath = filepath.Join(pluginDir, fmt.Sprintf("%s.sig", bin.Name))
			publicKeyPath = filepath.Join(pluginDir, "cosign.pub")
			if err := downloadFile(ctx, bin.SignatureURL, signaturePath, 0o600); err != nil {
				return m, err
			}
			if err := downloadFile(ctx, bin.PublicKeyURL, publicKeyPath, 0o600); err != nil {
				return m, err
			}
		}
		targetPath, err := downloadBinary(ctx, bin, pluginDir, signaturePath, publicKeyPath)
		if err != nil {
			return m, err
		}
		if err := VerifySHA256(targetPath, bin.Checksum); err != nil {
			return m, err
		}
		binaries[kind] = Binary{
			Path:      filepath.Base(targetPath),
			Checksum:  bin.Checksum,
			Signature: signaturePath,
			PublicKey: publicKeyPath,
		}
	}
	if len(binaries) == 0 {
		return m, fmt.Errorf("plugin registry entry has no binaries")
	}
	pluginType := versionEntry.Type
	if pluginType == "" {
		pluginType = TypeGRPC
	}
	protocol := versionEntry.Protocol
	if protocol == "" {
		protocol = ProtocolGRPC
	}
	if registryURL == "" {
		registryURL = defaultRegistryBaseURL
	}
	m = Manifest{
		Name:           name,
		Version:        versionEntry.Version,
		Type:           pluginType,
		Protocol:       protocol,
		CompatibleWith: versionEntry.CompatibleWith,
		Binaries:       binaries,
		Registry:       registryURL,
	}
	if err := SaveManifest(pluginDir, m); err != nil {
		return m, err
	}
	return m, nil
}

func selectVersion(versions []RegistryVersion, requested string) (RegistryVersion, error) {
	if requested != "" {
		for _, v := range versions {
			if v.Version == requested {
				return v, nil
			}
		}
		return RegistryVersion{}, fmt.Errorf("version %s not found", requested)
	}
	if len(versions) == 0 {
		return RegistryVersion{}, fmt.Errorf("no versions available")
	}
	var best RegistryVersion
	var bestVer *version.Version
	for _, v := range versions {
		parsed, err := version.NewVersion(v.Version)
		if err != nil {
			if bestVer == nil {
				best = v
			}
			continue
		}
		if bestVer == nil || parsed.GreaterThan(bestVer) {
			best = v
			bestVer = parsed
		}
	}
	if best.Version == "" {
		best = versions[len(versions)-1]
	}
	return best, nil
}

func downloadFile(ctx context.Context, srcURL, dest string, mode os.FileMode) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srcURL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download failed: %s", resp.Status)
	}
	tmp := dest + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}

func downloadBinary(ctx context.Context, bin RegistryBinary, pluginDir, signaturePath, publicKeyPath string) (string, error) {
	targetPath := filepath.Join(pluginDir, bin.Name)
	if isArchive(bin.URL) {
		archivePath := targetPath + ".archive"
		if err := downloadFile(ctx, bin.URL, archivePath, 0o600); err != nil {
			return "", err
		}
		if signaturePath != "" && publicKeyPath != "" {
			if err := VerifySignature(archivePath, signaturePath, publicKeyPath); err != nil {
				return "", err
			}
		}
		if err := extractFromArchive(archivePath, bin.Name, targetPath); err != nil {
			return "", err
		}
		_ = os.Remove(archivePath)
		return targetPath, nil
	}
	if err := downloadFile(ctx, bin.URL, targetPath, 0o700); err != nil {
		return "", err
	}
	if signaturePath != "" && publicKeyPath != "" {
		if err := VerifySignature(targetPath, signaturePath, publicKeyPath); err != nil {
			return "", err
		}
	}
	return targetPath, nil
}

func isArchive(url string) bool {
	lower := strings.ToLower(url)
	return strings.HasSuffix(lower, ".zip") || strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz")
}

func extractFromArchive(archivePath, fileName, targetPath string) error {
	if strings.HasSuffix(strings.ToLower(archivePath), ".zip") {
		return extractFromZip(archivePath, fileName, targetPath)
	}
	return extractFromTarGz(archivePath, fileName, targetPath)
}

func extractFromZip(archivePath, fileName, targetPath string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer func() {
		_ = reader.Close()
	}()
	for _, f := range reader.File {
		if filepath.Base(f.Name) != fileName {
			continue
		}
		src, err := f.Open()
		if err != nil {
			return err
		}
		defer func() {
			_ = src.Close()
		}()
		dst, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o700)
		if err != nil {
			return err
		}
		if _, err := io.Copy(dst, src); err != nil {
			_ = dst.Close()
			return err
		}
		if err := dst.Close(); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("file %s not found in archive", fileName)
}

func extractFromTarGz(archivePath, fileName, targetPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer func() {
		_ = gz.Close()
	}()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr == nil || filepath.Base(hdr.Name) != fileName {
			continue
		}
		dst, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o700)
		if err != nil {
			return err
		}
		if _, err := io.Copy(dst, tr); err != nil {
			_ = dst.Close()
			return err
		}
		if err := dst.Close(); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("file %s not found in archive", fileName)
}
