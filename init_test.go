package terrastack_test

import (
	"fmt"
	"io"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terrastack"
)

type testcase struct {
	stack   terrastack.Dirname
	force   bool
	wantErr error
}

var errorf = fmt.Errorf

func TestInit(t *testing.T) {
	afs := newFakeFS()
	terrastack.Setup(afs)

	for _, tc := range []testcase{
		{
			stack:   "/no/exists",
			force:   false,
			wantErr: errorf("init requires an existing directory"),
		},
		{
			stack: "/no/permission",
			force: false,
			wantErr: errorf("while checking \"/no/permission\": stat failed: " +
				"permission denied"),
		},
		{
			stack:   "/stack/initialized/same/version",
			force:   false,
			wantErr: nil,
		},
		{
			stack:   "/stack/initialized/same/version",
			force:   true,
			wantErr: nil,
		},
		{
			stack: "/stack/initialized/other/version",
			force: false,
			wantErr: errorf("stack initialized with other version 9999.9999.9999" +
				" (terrastack version is 0.0.1"),
		},
		{
			stack:   "/stack/initialized/other/version",
			force:   true,
			wantErr: nil,
		},
		{
			stack:   "/stack/not/initialized",
			force:   true,
			wantErr: nil,
		},
	} {
		err := terrastack.Init(tc.stack, tc.force)
		assert.EqualErrs(t, tc.wantErr, err)

		if err == nil {
			initFile, err := afs.Open(string(tc.stack) + "/terrastack")
			assert.NoError(t, err, "init file creation")

			data, err := io.ReadAll(initFile)
			assert.NoError(t, err, "init file read")
			assert.EqualStrings(t, terrastack.Version(), string(data))
		}
	}
}
