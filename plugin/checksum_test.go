// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"crypto/sha256"
	"fmt"
	"os"
	"testing"
)

func TestVerifySHA256Valid(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := dir + "/file"
	content := []byte("hello")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	sum := sha256.Sum256(content)
	expected := fmt.Sprintf("%x", sum[:])
	if err := VerifySHA256(path, expected); err != nil {
		t.Fatalf("expected valid checksum: %v", err)
	}
}

func TestVerifySHA256Invalid(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := dir + "/file"
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := VerifySHA256(path, "deadbeef"); err == nil {
		t.Fatalf("expected checksum error")
	}
}

func TestVerifySHA256Empty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := dir + "/file"
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := VerifySHA256(path, ""); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestVerifySHA256WithPrefix(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := dir + "/file"
	content := []byte("hello")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	sum := sha256.Sum256(content)
	expected := fmt.Sprintf("sha256:%x", sum[:])
	if err := VerifySHA256(path, expected); err != nil {
		t.Fatalf("expected valid checksum: %v", err)
	}
}
