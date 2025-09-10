// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package run

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/madlambda/spells/assert"
)

func TestLookPath_FindsExecutableInPATH(t *testing.T) {
	tmpDir := t.TempDir()
	binName := "tm-foo"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(tmpDir, binName)
	assert.NoError(t, os.WriteFile(binPath, []byte("#!/bin/sh\nexit 0\n"), 0o755))

	env := append([]string{}, os.Environ()...)
	// Prepend tmpDir to PATH
	pathKey := "PATH"
	if runtime.GOOS == "windows" {
		pathKey = "Path"
	}
	curPath, _ := Getenv(pathKey, env)
	newPath := tmpDir
	if curPath != "" {
		if runtime.GOOS == "windows" {
			newPath += ";" + curPath
		} else {
			newPath += ":" + curPath
		}
	}
	env = append(env, pathKey+"="+newPath)

	found, err := LookPath("tm-foo", env)
	assert.NoError(t, err)
	assert.EqualStrings(t, found, binPath)
}

func TestLookPath_AbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()
	binName := "tm-abs"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(tmpDir, binName)
	assert.NoError(t, os.WriteFile(binPath, []byte("#!/bin/sh\nexit 0\n"), 0o755))

	found, err := LookPath(binPath, os.Environ())
	assert.NoError(t, err)
	assert.EqualStrings(t, found, binPath)
}

func TestLookPath_NotFound(t *testing.T) {
	env := append([]string{}, os.Environ()...)
	// Ensure PATH is empty to avoid false positives
	env = append(env, "PATH=")

	_, err := LookPath("definitely-not-found-xyz", env)
	assert.Error(t, err)
}
