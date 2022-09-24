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

//go:build aix || android || darwin || dragonfly || freebsd || hurd || illumos || ios || linux || netbsd || openbsd || solaris

package hcl_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/test"
)

func TestFormatTreeFailsOnNonAccessibleSubdir(t *testing.T) {
	const subdir = "subdir"
	tmpdir := t.TempDir()
	test.Mkdir(t, tmpdir, subdir)

	assert.NoError(t, os.Chmod(filepath.Join(tmpdir, subdir), 0))

	_, err := hcl.FormatTree(tmpdir)
	assert.Error(t, err)
}

func TestFormatTreeFailsOnNonAccessibleFile(t *testing.T) {
	const filename = "filename.tm"

	tmpdir := t.TempDir()
	test.WriteFile(t, tmpdir, filename, `globals{
	a = 2
		b = 3
	}`)

	assert.NoError(t, os.Chmod(filepath.Join(tmpdir, filename), 0))

	_, err := hcl.FormatTree(tmpdir)
	assert.Error(t, err)
}
