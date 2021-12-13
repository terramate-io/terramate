// Copyright 2021 Mineiros GmbH
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

package terramate_test

import (
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate"
	"github.com/mineiros-io/terramate/test"
)

func TestGenerateFailsIfPathDoesntExist(t *testing.T) {
	assert.Error(t, terramate.Generate(test.NonExistingDir(t)))
}

func TestGenerateFailsIfPathIsNotDir(t *testing.T) {
	dir := t.TempDir()
	filename := "test"

	test.WriteFile(t, dir, filename, "whatever")
	path := filepath.Join(dir, filename)

	assert.Error(t, terramate.Generate(path))
}

func TestGenerateFailsIfPathIsRelative(t *testing.T) {
	dir := t.TempDir()
	relpath := test.RelPath(t, test.Getwd(t), dir)

	assert.Error(t, terramate.Generate(relpath))
}
