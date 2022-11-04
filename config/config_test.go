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

	_, found = cfg.Lookup("/non-existent")
	assert.IsTrue(t, !found)

	stacks := cfg.Stacks()
	assert.EqualInts(t, 3, len(stacks))
	assert.EqualStrings(t, "/stacks", project.PrjAbsPath(s.RootDir(), stacks[0].Dir()).String())
	assert.EqualStrings(t, "/stacks/child", project.PrjAbsPath(s.RootDir(), stacks[1].Dir()).String())
	assert.EqualStrings(t, "/stacks/child/non-stack/stack", project.PrjAbsPath(s.RootDir(), stacks[2].Dir()).String())
}

func TestConfigStacksByPaths(t *testing.T) {
	type testcase struct {
		name     string
		layout   []string
		basedir  string
		want     []string
		relpaths []string
	}

	for _, tc := range []testcase{
		{
			name:    "no stacks, no relpaths",
			layout:  []string{},
			basedir: "/",
		},
		{
			name:    "no stacks, absolute path",
			layout:  []string{},
			basedir: "/",
			relpaths: []string{
				"/",
			},
		},
		{
			name:    "no stacks, rel path",
			layout:  []string{},
			basedir: "/",
			relpaths: []string{
				"/",
			},
		},
		{
			name: "single stack, no matching path",
			layout: []string{
				"s:stack",
				"d:some/place",
			},
			basedir: "/some/place",
			relpaths: []string{
				"/test",
				"../../test",
			},
		},
		{
			name: "single stack, 1 matching absolute path",
			layout: []string{
				"s:stack",
				"d:some/place",
			},
			basedir: "/some/place",
			relpaths: []string{
				"/test",
				"../../test",
				"/stack",
			},
			want: []string{
				"/stack",
			},
		},
		{
			name: "single stack, 1 matching relpath",
			layout: []string{
				"s:stack",
				"d:some/place",
			},
			basedir: "/some/place",
			relpaths: []string{
				"/test",
				"../../test",
				"../../stack",
			},
			want: []string{
				"/stack",
			},
		},
		{
			name: "single stack, multiple match returns once",
			layout: []string{
				"s:stack",
				"d:some/place",
			},
			basedir: "/some/place",
			relpaths: []string{
				"../../stack",
				"../../stack",
			},
			want: []string{
				"/stack",
			},
		},
		{
			name: "single stack, 1 matching relpath",
			layout: []string{
				"s:stack",
				"d:some/place",
			},
			basedir: "/some/place",
			relpaths: []string{
				"/test",
				"../../test",
				"../../stack",
			},
			want: []string{
				"/stack",
			},
		},
		{
			name: "matching path returns all child stacks",
			layout: []string{
				"s:a",
				"s:a/lot",
				"s:a/lot/of",
				"s:a/lot/of/stacks",
				"d:a/lot/of/stacks/with-non-stack-dir",
				"s:a/lot/of/stacks/with-non-stack-dir/and",
				"s:a/lot/of/stacks/with-non-stack-dir/more",
				"s:a/lot/of/stacks/with-non-stack-dir/stacks",
				"d:b/is/not/stack",
				"s:b/is/not/stack/but-this-is-a-stack",
			},
			basedir: "/a/lot/of/stacks/with-non-stack-dir/and",
			relpaths: []string{
				"/",
			},
			want: []string{
				"/a",
				"/a/lot",
				"/a/lot/of",
				"/a/lot/of/stacks",
				"/a/lot/of/stacks/with-non-stack-dir/and",
				"/a/lot/of/stacks/with-non-stack-dir/more",
				"/a/lot/of/stacks/with-non-stack-dir/stacks",
				"/b/is/not/stack/but-this-is-a-stack",
			},
		},
		{
			name: "matching path returns all child stacks",
			layout: []string{
				"s:a",
				"s:a/lot",
				"s:a/lot/of",
				"s:a/lot/of/stacks",
				"d:a/lot/of/stacks/with-non-stack-dir",
				"s:a/lot/of/stacks/with-non-stack-dir/and",
				"s:a/lot/of/stacks/with-non-stack-dir/more",
				"s:a/lot/of/stacks/with-non-stack-dir/stacks",
				"d:b/is/not/stack",
				"s:b/is/not/stack/but-this-is-a-stack",
			},
			basedir: "/a/lot/of/stacks/with-non-stack-dir/and",
			relpaths: []string{
				"/",
			},
			want: []string{
				"/a",
				"/a/lot",
				"/a/lot/of",
				"/a/lot/of/stacks",
				"/a/lot/of/stacks/with-non-stack-dir/and",
				"/a/lot/of/stacks/with-non-stack-dir/more",
				"/a/lot/of/stacks/with-non-stack-dir/stacks",
				"/b/is/not/stack/but-this-is-a-stack",
			},
		},
	} {
		s := sandbox.New(t)
		s.BuildTree(tc.layout)
		cfg := s.Config()
		got := cfg.StacksByPaths(project.NewPath(tc.basedir), tc.relpaths...)
		assert.EqualInts(t, len(tc.want), len(got))
		var stacks []string
		for _, node := range got {
			stacks = append(stacks, project.PrjAbsPath(cfg.RootDir(), node.Dir()).String())
		}
		for i, want := range tc.want {
			assert.EqualStrings(t, want, stacks[i])
		}
	}
}

func TestConfigSkipdir(t *testing.T) {
	s := sandbox.New(t)
	s.BuildTree([]string{
		"s:/stack",
		"s:/stack-2",
		"f:/stack/" + config.SkipFilename,
		"f:/stack/ignored.tm:not valid hcl but wont be parsed",
		"f:/stack/subdir/ignored.tm:not valid hcl but wont be parsed",
	})

	cfg, err := config.LoadRoot(s.RootDir())
	assert.NoError(t, err)

	node, found := cfg.Lookup("/stack-2")
	assert.IsTrue(t, found)
	assert.IsTrue(t, !node.IsEmptyConfig())
	assert.IsTrue(t, node.IsStack())

	// When we find a tmskip the node is created but empty, no parsing is done
	node, found = cfg.Lookup("/stack")
	assert.IsTrue(t, found)
	assert.IsTrue(t, node.IsEmptyConfig())
	assert.IsTrue(t, !node.IsStack())

	// subdirs are not processed and can't be found
	_, found = cfg.Lookup("/stack/subdir")
	assert.IsTrue(t, !found)
}

func isStack(root *config.Root, dir string) bool {
	return config.IsStack(root, filepath.Join(root.RootDir(), dir))
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
