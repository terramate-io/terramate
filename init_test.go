package terrastack_test

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terrastack"
	"github.com/mineiros-io/terrastack/hcl"
	"github.com/mineiros-io/terrastack/hcl/hhcl"
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
		err := terrastack.Init(stackdir, tc.force)
		assert.EqualErrs(t, tc.wantErr, err)

		if err == nil {
			initFile, err := os.Open(filepath.Join(stackdir,
				terrastack.ConfigFilename))
			assert.NoError(t, err, "init file creation")

			data, err := io.ReadAll(initFile)
			assert.NoError(t, err, "init file read")
			assertTSBlock(t, hcl.Terrastack{
				RequiredVersion: terrastack.Version(),
			}, string(data))
		}
	}
}

func sameVersionStack(t *testing.T) string {
	stack := t.TempDir()
	_ = test.WriteFile(t, stack, terrastack.ConfigFilename, fmt.Sprintf(`
terrastack {
	required_version = "~> %s"
}
`, terrastack.Version()))
	return stack
}

func otherVersionStack(t *testing.T) string {
	stack := t.TempDir()
	_ = test.WriteFile(t, stack, terrastack.ConfigFilename, fmt.Sprintf(`
terrastack {
	required_version = "~> %s"
}
`, "9999.9999.9999"))
	return stack
}

func newStack(t *testing.T) string {
	return t.TempDir()
}

func assertTSBlock(t *testing.T, want hcl.Terrastack, got string) {
	p := hhcl.NewParser()
	ts, err := p.Parse("test", []byte(got))
	assert.NoError(t, err, "parsing generated file")

	if *ts != want {
		t.Fatalf("terrastack file mismatch. %+v != %+v", *ts, want)
	}
}
