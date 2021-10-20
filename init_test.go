package terrastack_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terrastack"
)

type dirgen func(t *testing.T) string

type testcase struct {
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

	for _, tc := range []testcase{
		{
			stack:   nonExistentDir,
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

		err := terrastack.Init(stackdir, tc.force)
		assert.EqualErrs(t, tc.wantErr, err)

		if err == nil {
			initFile, err := os.Open(string(stackdir) + "/terrastack")
			assert.NoError(t, err, "init file creation")

			data, err := io.ReadAll(initFile)
			assert.NoError(t, err, "init file read")
			assert.EqualStrings(t, terrastack.Version(), string(data))
		}
	}
}

func tempdir(t *testing.T, base string) string {
	tmp, err := ioutil.TempDir(base, "terrastack-test-dir-")
	assert.NoError(t, err, "creating tempdir")

	return tmp
}

func nonExistentDir(t *testing.T) string {
	tmp := tempdir(t, "")
	tmp2 := tempdir(t, tmp)

	assert.NoError(t, os.RemoveAll(tmp2), "remove directory")

	return tmp2
}

func sameVersionStack(t *testing.T) string {
	stack := tempdir(t, "")
	stackfile := filepath.Join(stack, "terrastack")

	err := ioutil.WriteFile(stackfile, []byte(terrastack.Version()), 0644)
	assert.NoError(t, err, "write same version stackfile")

	return stack
}

func otherVersionStack(t *testing.T) string {
	stack := tempdir(t, "")
	stackfile := filepath.Join(stack, "terrastack")

	err := ioutil.WriteFile(stackfile, []byte("9999.9999.9999"), 0644)
	assert.NoError(t, err, "write other version stackfile")

	return stack
}

func newStack(t *testing.T) string {
	return tempdir(t, "")
}

func removeStack(t *testing.T, stackdir string) {
	assert.NoError(t, os.RemoveAll(stackdir), "removing stack \"%s\"", stackdir)
}
