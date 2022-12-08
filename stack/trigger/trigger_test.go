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
	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/stack/trigger"
	"github.com/mineiros-io/terramate/test"
	"github.com/rs/zerolog"
)

func TestTriggerForStackOnRoot(t *testing.T) {
	testTrigger(t, project.NewPath("/stack"), "stack inside root")
}

func TestTriggerForStackOnSubdir(t *testing.T) {
	testTrigger(t, project.NewPath("/dir/stack"), "subdir stack")
}

func TestTriggerForRootIsStack(t *testing.T) {
	testTrigger(t, project.NewPath("/"), "root is stack")
}

func testTrigger(t *testing.T, path project.Path, reason string) {
	rootdir := t.TempDir()

	err := trigger.Create(rootdir, path, reason)
	assert.NoError(t, err)

	// check created trigger on fs
	triggerDir := filepath.Join(trigger.Dir(rootdir), path.String())
	entries := test.ReadDir(t, triggerDir)
	if len(entries) != 1 {
		t.Fatalf("want 1 trigger file, got %d: %+v", len(entries), entries)
	}

	triggerFile := filepath.Join(triggerDir, entries[0].Name())
	triggerInfo, err := trigger.ParseFile(triggerFile)

	assert.NoError(t, err)
	assert.EqualStrings(t, reason, triggerInfo.Reason)

	assert.IsTrue(t, triggerInfo.Ctime > 0)
	assert.IsTrue(t, triggerInfo.Ctime < math.MaxInt64)
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
