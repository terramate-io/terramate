package trigger_test

import (
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
	// TODO: PARSE CTIME
	//assert.IsTrue(t, triggerInfo.Ctime > 0)
	//assert.IsTrue(t, triggerInfo.Ctime < math.MaxInt64)
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
