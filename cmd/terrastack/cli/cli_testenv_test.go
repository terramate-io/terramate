package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mineiros-io/terrastack/test"
)

type TestEnv struct {
	t       *testing.T
	basedir string
}

// DirEntry represents a directory and can be
// used to create files inside the directory
type DirEntry struct {
	t    *testing.T
	path string
}

// FileEntry represents a file and can be used
// to manipulate the file contents.
// It is optimized for reading/writing all contents,
// not stream programming (io.Reader/io.Writer).
// It has limited usefulness but it is easier to
// work with for testing.
type FileEntry struct {
	t    *testing.T
	path string
}

// NewTestEnv creates a new test env, including a new
// temporary repository. All commands run using the test
// env will use this tmp dir as the working dir.
//
// It is a programming error to use a test env created
// with a *testing.T other than the one of the test
// using the test env, for a new test/sub-test always create
// a new test env for it.
func NewTestEnv(t *testing.T) *TestEnv {
	t.Helper()
	return &TestEnv{
		t:       t,
		basedir: test.TempDir(t, ""),
	}
}

// Cleanup will release any resources, like files, created
// by the test env, it is a programming error to use the test
// env (or any object created from it) after calling this method.
func (te *TestEnv) Cleanup() {
	te.t.Helper()

	test.RemoveAll(te.t, te.basedir)
}

// Run will run the given cmd with the provided args using
// the test env base dir as the command working dir.
// This method fails the test if the command fails, where
// a command failed is defined by its status code (!= 0).
func (te *TestEnv) Run(name string, args ...string) {
	te.t.Helper()

	cmd := exec.Command(name, args...)
	cmd.Dir = te.basedir

	if out, err := cmd.CombinedOutput(); err != nil {
		te.t.Errorf("failed to run: '%v' err: '%v' output: '%s'", cmd, err, string(out))
	}
}

// CreateModule will create a module dir with the given name
// returning a directory entry that can be used to
// create files inside the module dir.
//
// It is a programming error to call this method with a module
// name that was already created before.
func (te *TestEnv) CreateModule(name string) *DirEntry {
	te.t.Helper()

	moddir := filepath.Join(te.basedir, "modules", name)
	test.MkdirAll(te.t, moddir)

	return &DirEntry{t: te.t, path: moddir}
}

// CreateFile will create a file inside the dir with the
// given name and the given contents.
//
// It returns a file entry that can be used to further
// manipulate the created file, like replacing its contents.
// The file entry is optimized for always replacing the
// file contents, not streaming (using file as io.Writer).
//
// If the file already exists its contents will be truncated,
// like os.Create behavior: https://pkg.go.dev/os#Create
func (de *DirEntry) CreateFile(name, body string) *FileEntry {
	de.t.Helper()

	fe := &FileEntry{
		t:    de.t,
		path: filepath.Join(de.path, name),
	}
	fe.Write(body)

	return fe
}

// Write writes the given text body on the file, replacing its contents.
// It behaves like os.WriteFile: https://pkg.go.dev/os#WriteFile
func (fe *FileEntry) Write(body string) {
	fe.t.Helper()

	if err := os.WriteFile(fe.path, []byte(body), 0700); err != nil {
		fe.t.Errorf("os.WriteFile(%q) = %v", fe.path, err)
	}
}
