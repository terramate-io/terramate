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

package trigger_test

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/stack/trigger"
	"github.com/mineiros-io/terramate/test"
	errtest "github.com/mineiros-io/terramate/test/errors"
	"github.com/mineiros-io/terramate/test/sandbox"
	"github.com/rs/zerolog"
)

type testcase struct {
	name   string
	layout []string
	path   string
	reason string
	want   error
}

func TestTriggerStacks(t *testing.T) {
	t.Parallel()
	for _, tc := range []testcase{
		{
			name: "stack on root",
			layout: []string{
				"s:stack",
			},
			path:   "/stack",
			reason: "stack inside root",
		},
		{
			name: "stack on subdir",
			layout: []string{
				"s:dir/stack",
			},
			path:   "/dir/stack",
			reason: "subdir stack",
		},
		{
			name: "root is stack",
			layout: []string{
				"s:.",
			},
			path:   "/",
			reason: "root is stack",
		},
		{
			name:   "stack doesnt exist",
			path:   "/non-existent-stack",
			reason: "should not trigger",
			want:   errors.E(trigger.ErrTrigger),
		},
		{
			name: "subdir of a stack is not valid",
			path: "/stack/dir",
			layout: []string{
				"s:stack",
				"d:stack/dir",
			},
			want: errors.E(trigger.ErrTrigger),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testTrigger(t, tc)
		})
	}
}

func testTrigger(t *testing.T, tc testcase) {
	s := sandbox.New(t)
	s.BuildTree(tc.layout)
	root, err := config.LoadRoot(s.RootDir())
	assert.NoError(t, err)
	err = trigger.Create(root, project.NewPath(tc.path), tc.reason)
	errtest.Assert(t, err, tc.want)

	if err != nil {
		return
	}

	// check created trigger on fs
	triggerDir := filepath.Join(trigger.Dir(root.Dir()), tc.path)
	entries := test.ReadDir(t, triggerDir)
	if len(entries) != 1 {
		t.Fatalf("want 1 trigger file, got %d: %+v", len(entries), entries)
	}

	triggerFile := filepath.Join(triggerDir, entries[0].Name())
	triggerInfo, err := trigger.ParseFile(triggerFile)

	assert.NoError(t, err)
	assert.EqualStrings(t, tc.reason, triggerInfo.Reason)

	assert.IsTrue(t, triggerInfo.Ctime > 0)
	assert.IsTrue(t, triggerInfo.Ctime < math.MaxInt64)

	gotPath, ok := trigger.StackPath(project.PrjAbsPath(root.Dir(), triggerFile))

	assert.IsTrue(t, ok)
	assert.EqualStrings(t, tc.path, gotPath.String())
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
