// Copyright 2022 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package modvendor_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/modvendor"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/sandbox"
	"github.com/mineiros-io/terramate/tf"
	"github.com/rs/zerolog"
)

func TestModVendorWithRef(t *testing.T) {
	const (
		path     = "github.com/mineiros-io/example"
		ref      = "main"
		filename = "test.txt"
		content  = "test"
	)

	s := sandbox.New(t)

	s.RootEntry().CreateFile(filename, content)

	g := s.Git()
	g.CommitAll("add file")

	gitURL := "file://" + s.RootDir()
	vendorDir := t.TempDir()

	cloneDir, err := modvendor.Vendor(vendorDir, tf.Source{
		URL:  gitURL,
		Ref:  ref,
		Path: path,
	})
	assert.NoError(t, err)

	wantCloneDir := filepath.Join(vendorDir, path, ref)
	assert.EqualStrings(t, wantCloneDir, cloneDir)

	got := test.ReadFile(t, cloneDir, filename)
	assert.EqualStrings(t, content, string(got))

	const (
		newRef      = "branch"
		newFilename = "new.txt"
		newContent  = "new"
	)

	g.CheckoutNew(newRef)
	s.RootEntry().CreateFile(newFilename, newContent)
	g.CommitAll("add new file")

	newCloneDir, err := modvendor.Vendor(vendorDir, tf.Source{
		URL:  gitURL,
		Ref:  newRef,
		Path: path,
	})
	assert.NoError(t, err)

	wantCloneDir = filepath.Join(vendorDir, path, newRef)
	assert.EqualStrings(t, wantCloneDir, newCloneDir)

	got = test.ReadFile(t, newCloneDir, filename)
	assert.EqualStrings(t, content, string(got))

	got = test.ReadFile(t, newCloneDir, newFilename)
	assert.EqualStrings(t, newContent, string(got))
}

func TestModVendorDoesNothingIfRefExists(t *testing.T) {
	const (
		path = "github.com/mineiros-io/example"
		ref  = "main"
	)

	s := sandbox.New(t)

	s.RootEntry().CreateFile("file.txt", "data")

	g := s.Git()
	g.CommitAll("add file")

	gitURL := "file://" + s.RootDir()
	vendordir := t.TempDir()
	clonedir := filepath.Join(vendordir, path, ref)
	test.MkdirAll(t, clonedir)

	_, err := modvendor.Vendor(vendordir, tf.Source{
		URL:  gitURL,
		Ref:  ref,
		Path: path,
	})
	assert.IsError(t, err, errors.E(modvendor.ErrAlreadyVendored))

	entries, err := os.ReadDir(clonedir)
	assert.NoError(t, err)

	if len(entries) > 0 {
		t.Fatalf("wanted clone dir to be empty, got: %v", entries)
	}
}

func TestModVendorNoRefFails(t *testing.T) {
	// TODO(katcipis): when we start parsing modules for sources
	// we need to address default remote references. For now it is
	// always explicit.
	const (
		path = "github.com/mineiros-io/example"
	)

	s := sandbox.New(t)
	gitURL := "file://" + s.RootDir()
	vendorDir := t.TempDir()

	_, err := modvendor.Vendor(vendorDir, tf.Source{
		URL:  gitURL,
		Path: path,
	})

	assert.Error(t, err)
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
