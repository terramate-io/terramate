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

package generate_test

import (
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/generate"
	"github.com/mineiros-io/terramate/test"
)

func TestCheckFailsIfPathDoesntExist(t *testing.T) {
	_, err := generate.Check(test.NonExistingDir(t))
	assert.Error(t, err)
}

func TestCheckFailsIfPathIsNotDir(t *testing.T) {
	dir := t.TempDir()
	filename := "test"

	test.WriteFile(t, dir, filename, "whatever")
	path := filepath.Join(dir, filename)

	_, err := generate.Check(path)
	assert.Error(t, err)
}

func TestCheckFailsIfPathIsRelative(t *testing.T) {
	dir := t.TempDir()
	relpath := test.RelPath(t, test.Getwd(t), dir)

	_, err := generate.Check(relpath)
	assert.Error(t, err)
}
