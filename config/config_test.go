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

package config_test

import (
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/test/sandbox"
	"github.com/rs/zerolog"
)

func TestIsStack(t *testing.T) {
	s := sandbox.New(t)
	s.BuildTree([]string{
		"d:/dir",
		"s:/stack",
		"d:/stack/subdir",
	})

	assert.IsTrue(t, !isStack(t, s.RootDir(), "/dir"))
	assert.IsTrue(t, isStack(t, s.RootDir(), "/stack"))
	assert.IsTrue(t, !isStack(t, s.RootDir(), "/stack/subdir"))
}

func isStack(t *testing.T, rootdir, dir string) bool {
	t.Helper()

	res, err := config.IsStack(rootdir, filepath.Join(rootdir, dir))
	assert.NoError(t, err)

	return res
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
