// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package config_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/rs/zerolog"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestIsStack(t *testing.T) {
	t.Parallel()
	s := sandbox.NoGit(t, true)
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

func TestValidStackIDs(t *testing.T) {
	t.Parallel()
	validIDs := []string{
		"_",
		"-",
		"_id_",
		"-id-",
		"_id_again_",
		"-id-again-",
		"-id_mixed-",
		"-id_numbers-0123456789-",
		"maxsize_id_Test_should_Be_64_bytes_aNd_now_running_out_of_ID-aaa",
	}
	invalidIDs := []string{
		"*not+valid$",
		"cachaça",
		"maxsize_id_test_should_be_64_bytes_and_now_running_out_of_id-aaac",
	}

	for _, validID := range validIDs {
		validID := validID
		t.Run(fmt.Sprintf("valid ID %s", validID), func(t *testing.T) {
			t.Parallel()
			s := sandbox.NoGit(t, true)
			s.BuildTree([]string{
				"s:stack:id=" + validID,
			})
			root, err := config.LoadRoot(s.RootDir())
			assert.NoError(t, err)
			stacknode, ok := root.Lookup(project.NewPath("/stack"))
			assert.IsTrue(t, ok && stacknode.IsStack())
		})
	}

	for _, invalidID := range invalidIDs {
		invalidID := invalidID
		t.Run(fmt.Sprintf("invalid ID %s", invalidID), func(t *testing.T) {
			t.Parallel()
			s := sandbox.NoGit(t, true)
			s.BuildTree([]string{
				"s:stack:id=" + invalidID,
			})
			root, err := config.LoadRoot(s.RootDir())
			assert.NoError(t, err)
			_, err = config.LoadStack(root, project.NewPath("/stack"))
			assert.IsError(t, err, errors.E(config.ErrStackValidation))
		})
	}
}

func TestConfigLookup(t *testing.T) {
	t.Parallel()
	s := sandbox.NoGit(t, true)
	s.BuildTree([]string{
		"d:/dir",
		"s:/stacks",
		"s:/stacks/child",
		"s:/stacks/child/non-stack/stack",
	})

	root := s.Config()
	node, found := root.Lookup(project.NewPath("/dir"))
	assert.IsTrue(t, found)
	assert.IsTrue(t, node.IsEmptyConfig())

	node, found = root.Lookup(project.NewPath("/stacks"))
	assert.IsTrue(t, found && node.IsStack() && !node.IsEmptyConfig())

	node, found = root.Lookup(project.NewPath("/stacks/child"))
	assert.IsTrue(t, found && node.IsStack() && !node.IsEmptyConfig())

	node, found = root.Lookup(project.NewPath("/stacks/child/non-stack"))
	assert.IsTrue(t, found)
	assert.IsTrue(t, node.IsEmptyConfig())

	node, found = root.Lookup(project.NewPath("/stacks/child/non-stack/stack"))
	assert.IsTrue(t, found && node.IsStack() && !node.IsEmptyConfig())

	_, found = root.Lookup(project.NewPath("/non-existent"))
	assert.IsTrue(t, !found)

	stacks := root.Tree().Stacks()
	assert.EqualInts(t, 3, len(stacks))
	assert.EqualStrings(t, "/stacks", stacks[0].Dir().String())
	assert.EqualStrings(t, "/stacks/child", stacks[1].Dir().String())
	assert.EqualStrings(t, "/stacks/child/non-stack/stack", stacks[2].Dir().String())
}

func TestConfigStacksByPaths(t *testing.T) {
	t.Parallel()
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
		s := sandbox.NoGit(t, true)
		s.BuildTree(tc.layout)
		root := s.Config()
		got := root.StacksByPaths(project.NewPath(tc.basedir), tc.relpaths...)
		assert.EqualInts(t, len(tc.want), len(got))
		var stacks []string
		for _, node := range got {
			stacks = append(stacks, node.Dir().String())
		}
		for i, want := range tc.want {
			assert.EqualStrings(t, want, stacks[i])
		}
	}
}

func TestConfigSkipdir(t *testing.T) {
	t.Parallel()
	s := sandbox.NoGit(t, true)
	s.BuildTree([]string{
		"s:/stack",
		"s:/stack-2",
		"f:/stack/" + config.SkipFilename,
		"f:/stack/ignored.tm:not valid hcl but wont be parsed",
		"f:/stack/subdir/ignored.tm:not valid hcl but wont be parsed",
	})

	root, err := config.LoadRoot(s.RootDir())
	assert.NoError(t, err)

	node, found := root.Lookup(project.NewPath("/stack-2"))
	assert.IsTrue(t, found)
	assert.IsTrue(t, !node.IsEmptyConfig())
	assert.IsTrue(t, node.IsStack())

	// When we find a tmskip the node is created but empty, no parsing is done
	node, found = root.Lookup(project.NewPath("/stack"))
	assert.IsTrue(t, found)
	assert.IsTrue(t, node.IsEmptyConfig())
	assert.IsTrue(t, !node.IsStack())

	// subdirs are not processed and can't be found
	_, found = root.Lookup(project.NewPath("/stack/subdir"))
	assert.IsTrue(t, !found)
}

func isStack(root *config.Root, dir string) bool {
	return config.IsStack(root, filepath.Join(root.HostDir(), dir))
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
