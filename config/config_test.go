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
	"github.com/mineiros-io/terramate/project"
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

	cfg := s.Config()
	assert.IsTrue(t, !isStack(cfg, "/dir"))
	assert.IsTrue(t, isStack(cfg, "/stack"))
	assert.IsTrue(t, !isStack(cfg, "/stack/subdir"))
}

func TestConfigLookup(t *testing.T) {
	s := sandbox.New(t)
	s.BuildTree([]string{
		"d:/dir",
		"s:/stacks",
		"s:/stacks/child",
		"s:/stacks/child/non-stack/stack",
	})

	cfg := s.Config()
	node, found := cfg.Lookup("/dir")
	assert.IsTrue(t, found)
	assert.IsTrue(t, node.IsEmptyConfig())

	node, found = cfg.Lookup("/stacks")
	assert.IsTrue(t, found && node.IsStack() && !node.IsEmptyConfig())

	node, found = cfg.Lookup("/stacks/child")
	assert.IsTrue(t, found && node.IsStack() && !node.IsEmptyConfig())

	node, found = cfg.Lookup("/stacks/child/non-stack")
	assert.IsTrue(t, found)
	assert.IsTrue(t, node.IsEmptyConfig())

	node, found = cfg.Lookup("/stacks/child/non-stack/stack")
	assert.IsTrue(t, found && node.IsStack() && !node.IsEmptyConfig())

	_, found = cfg.Lookup("/non-existant")
	assert.IsTrue(t, !found)

	stacks := cfg.Stacks()
	assert.EqualInts(t, 3, len(stacks))
	assert.EqualStrings(t, "/stacks", project.PrjAbsPath(s.RootDir(), stacks[0].RootDir()).String())
	assert.EqualStrings(t, "/stacks/child", project.PrjAbsPath(s.RootDir(), stacks[1].RootDir()).String())
	assert.EqualStrings(t, "/stacks/child/non-stack/stack", project.PrjAbsPath(s.RootDir(), stacks[2].RootDir()).String())
}

func isStack(cfg *config.Tree, dir string) bool {
	return config.IsStack(cfg, filepath.Join(cfg.RootDir(), dir))
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
