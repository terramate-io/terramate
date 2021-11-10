package terrastack_test

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terrastack"
	"github.com/mineiros-io/terrastack/test"
)

type dirgen func(t *testing.T) string

type initTestcase struct {
	stack   dirgen
	force   bool
	wantErr error
}

var errorf = fmt.Errorf

func TestInit(t *testing.T) {
	allstacks := []string{}

	defer func() {
		for _, d := range allstacks {
			removeStack(t, d)
		}
	}()

	for _, tc := range []initTestcase{
		{
			stack:   test.NonExistingDir,
			force:   false,
			wantErr: errorf("init requires an existing directory"),
		},
		{
			stack:   sameVersionStack,
			force:   false,
			wantErr: nil,
		},
		{
			stack:   sameVersionStack,
			force:   true,
			wantErr: nil,
		},
		{
			stack: otherVersionStack,
			force: false,
			wantErr: errorf("stack already initialized with version " +
				"\"9999.9999.9999\" but terrastack version is \"0.0.1\""),
		},
		{
			stack:   otherVersionStack,
			force:   true,
			wantErr: nil,
		},
		{
			stack:   newStack,
			force:   true,
			wantErr: nil,
		},
	} {
		stackdir := tc.stack(t)

		allstacks = append(allstacks, stackdir)

		mgr := terrastack.NewManager(stackdir)
		err := mgr.Init(stackdir, tc.force)
		assert.EqualErrs(t, tc.wantErr, err)

		if err == nil {
			initFile, err := os.Open(filepath.Join(stackdir,
				terrastack.ConfigFilename))
			assert.NoError(t, err, "init file creation")

			data, err := io.ReadAll(initFile)
			assert.NoError(t, err, "init file read")
			assert.EqualStrings(t, terrastack.Version(), string(data))
		}

		removeStack(t, stackdir)
	}
}

func sameVersionStack(t *testing.T) string {
	stack := test.TempDir(t, "")
	_ = test.WriteFile(t, stack, terrastack.ConfigFilename, terrastack.Version())
	return stack
}

func otherVersionStack(t *testing.T) string {
	stack := test.TempDir(t, "")
	_ = test.WriteFile(t, stack, terrastack.ConfigFilename, "9999.9999.9999")
	return stack
}

func newStack(t *testing.T) string {
	return test.TempDir(t, "")
}

func removeStack(t *testing.T, stackdir string) {
	assert.NoError(t, os.RemoveAll(stackdir), "removing stack %q", stackdir)
}
