// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package generate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"

	"github.com/terramate-io/terramate/config"
	genreport "github.com/terramate-io/terramate/generate/report"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/project"
)

// The order of bundles and their stacks is not deterministic, so this test sets up the
// registry directly to ensure a nested stack is created before its parent.
func TestGenerateBundleStacksNestedBeforeParent(t *testing.T) {
	t.Parallel()

	rootdir := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(rootdir, "terramate.tm.hcl"), []byte("terramate {}\n"), 0644))

	root, err := config.LoadRoot(rootdir, false)
	assert.NoError(t, err)

	newBundle := func(name, dir, comp string) *config.Bundle {
		stackDir := project.NewPath(dir)
		return &config.Bundle{
			Name:   name,
			Source: "/bundles/" + name,
			Stacks: map[project.Path]config.StackMetadata{
				stackDir: {
					Dir:        stackDir,
					Name:       name,
					Components: []*hcl.Component{{Name: comp}},
				},
			},
		}
	}

	g := &gState{
		root: root,
		registry: &config.Registry{
			Bundles: []*config.Bundle{
				newBundle("child", "/parent/child", "child-comp"),
				newBundle("parent", "/parent", "parent-comp"),
			},
		},
	}

	report := genreport.Report{}
	g.generateBundleStacks(&report, true)
	assert.IsTrue(t, !report.HasFailures(), "unexpected failures: %v", report.Failures)

	for _, want := range []struct {
		dir  string
		name string
		comp string
	}{
		{dir: "/parent", name: "parent", comp: "parent-comp"},
		{dir: "/parent/child", name: "child", comp: "child-comp"},
	} {
		tree, ok := root.Lookup(project.NewPath(want.dir))
		assert.IsTrue(t, ok && tree.IsStack(), "stack %s not found", want.dir)
		assert.EqualStrings(t, want.name, tree.Node.Stack.Name)
		assert.EqualInts(t, 1, len(tree.Node.Components), "components of stack %s", want.dir)
		assert.EqualStrings(t, want.comp, tree.Node.Components[0].Name)
	}
}
