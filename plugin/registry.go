// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"runtime"
	"time"
)

const defaultRegistryBaseURL = "https://plugins.terramate.io/v1"

// RegistryBinary describes a binary download entry.
type RegistryBinary struct {
	Name         string `json:"name"`
	URL          string `json:"url"`
	Checksum     string `json:"checksum,omitempty"`
	SignatureURL string `json:"signature_url,omitempty"`
	PublicKeyURL string `json:"public_key_url,omitempty"`
}

// RegistryAsset describes binaries for a specific OS/arch.
type RegistryAsset struct {
	OS       string                        `json:"os"`
	Arch     string                        `json:"arch"`
	Binaries map[BinaryKind]RegistryBinary `json:"binaries"`
}

// RegistryVersion describes a plugin version entry.
type RegistryVersion struct {
	Version        string          `json:"version"`
	CompatibleWith string          `json:"compatible_with,omitempty"`
	Type           Type            `json:"type"`
	Protocol       Protocol        `json:"protocol,omitempty"`
	Assets         []RegistryAsset `json:"assets"`
}

// RegistryIndex describes the available versions for a plugin.
type RegistryIndex struct {
	Name     string            `json:"name"`
	Versions []RegistryVersion `json:"versions"`
}

// RegistryClient fetches plugin metadata.
type RegistryClient struct {
	baseURL string
	client  *http.Client
}

// NewRegistryClient creates a registry client.
func NewRegistryClient(baseURL string) *RegistryClient {
	if baseURL == "" {
		baseURL = defaultRegistryBaseURL
	}
	return &RegistryClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// FetchIndex fetches the registry index for a plugin.
func (c *RegistryClient) FetchIndex(ctx context.Context, name string) (RegistryIndex, error) {
	var idx RegistryIndex
	regURL, err := c.indexURL(name)
	if err != nil {
		return idx, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, regURL, nil)
	if err != nil {
		return idx, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return idx, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return idx, fmt.Errorf("registry request failed: %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(&idx); err != nil {
		return idx, err
	}
	return idx, nil
}

func (c *RegistryClient) indexURL(name string) (string, error) {
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return "", err
	}
	base.Path = path.Join(base.Path, "plugins", name, "versions.json")
	return base.String(), nil
}

// SelectAsset returns the asset that matches the current platform.
func (v RegistryVersion) SelectAsset() (RegistryAsset, bool) {
	for _, asset := range v.Assets {
		if asset.OS == runtime.GOOS && asset.Arch == runtime.GOARCH {
			return asset, true
		}
	}
	return RegistryAsset{}, false
}
