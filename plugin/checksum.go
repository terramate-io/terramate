// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package plugin provides plugin installation helpers.
package plugin

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strings"
)

// VerifySHA256 validates the SHA256 checksum of a file against an expected value.
func VerifySHA256(path, expected string) error {
	if expected == "" {
		return nil
	}
	expected = strings.TrimSpace(expected)
	expected = strings.TrimPrefix(expected, "sha256:")
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return err
	}
	actual := fmt.Sprintf("%x", hasher.Sum(nil))
	if actual != expected {
		return fmt.Errorf("checksum mismatch: expected %s got %s", expected, actual)
	}
	return nil
}
